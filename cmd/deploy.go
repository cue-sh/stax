package cmd

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/graph"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&flags.DeployWait, "wait", "w", false, "Wait for stack updates to complete before continuing.")
	deployCmd.Flags().BoolVarP(&flags.DeploySave, "save", "s", false, "Save stack outputs upon successful completion. Implies --wait.")
	deployCmd.Flags().BoolVarP(&flags.DeployDeps, "dependencies", "d", false, "Deploy stack dependencies in order. Implies --save.")
	deployCmd.Flags().BoolVarP(&flags.DeployPrevious, "previous-values", "v", false, "Deploy stack using previous parameter values.")
}

type deployArgs struct {
	stack         stx.Stack
	stackValue    cue.Value
	buildInstance *build.Instance
}

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys a stack by creating a changeset, previews expected changes, and optionally executes.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()
		stx.EnsureVaultSession(config)

		if flags.DeployDeps {
			flags.DeploySave = true
		}

		availableStacks := make(map[string]deployArgs)
		workingGraph := graph.NewGraph()
		buildInstances := stx.GetBuildInstances(args, "cfn")

		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := stx.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			// since the Process handler generally only sees one stack per instance,
			// we need to gather ALL the stacks first primarily to support dependencies
			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack stx.Stack
				decodeErr := stackValue.Decode(&stack)
				if decodeErr != nil {
					if flags.DeployDeps {
						log.Fatal(decodeErr)
					} else {
						log.Error(decodeErr)
						continue
					}
				}

				availableStacks[stack.Name] = deployArgs{stack: stack, buildInstance: buildInstance, stackValue: stackValue}
				if flags.DeployDeps {
					workingGraph.AddNode(stack.Name, stack.DependsOn...)
				}
			}
		})

		if flags.DeployDeps {
			resolved, err := workingGraph.Resolve()
			if err != nil {
				log.Fatalf("Failed to resolve dependency graph: %s\n", err)
			}

			for _, stackName := range resolved {
				dplArgs := availableStacks[stackName]
				deployStack(dplArgs.stack, dplArgs.buildInstance, dplArgs.stackValue)
			}
		} else {
			for _, dplArgs := range availableStacks {
				deployStack(dplArgs.stack, dplArgs.buildInstance, dplArgs.stackValue)
			}
		}
	},
}

func deployStack(stack stx.Stack, buildInstance *build.Instance, stackValue cue.Value) {

	fileName, saveErr := saveStackAsYml(stack, buildInstance, stackValue)
	if saveErr != nil {
		log.Error(saveErr)
	}
	log.Infof("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stack.Name), au.White("⤏"), au.Green(stack.Profile), au.Cyan(stack.Region))
	log.Infof("%s", au.Gray(11, "  Validating template..."))

	// get a session and cloudformation service client
	log.Debugf("\nGetting session for profile %s\n", stack.Profile)
	session := stx.GetSession(stack.Profile)
	cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

	// read template from disk
	log.Debug("Reading template from", fileName)
	templateFileBytes, _ := ioutil.ReadFile(fileName)
	templateBody := string(templateFileBytes)
	usr, _ := user.Current()

	changeSetName := "stx-dpl-" + usr.Username + "-" + fmt.Sprintf("%x", sha1.Sum(templateFileBytes))
	// validate template
	validateTemplateInput := cloudformation.ValidateTemplateInput{
		TemplateBody: &templateBody,
	}
	validateTemplateOutput, validateTemplateErr := cfn.ValidateTemplate(&validateTemplateInput)

	// template failed to validate
	if validateTemplateErr != nil {
		log.Infof(" %s\n", au.Red("✕"))
		log.Fatalf("%+v\n", validateTemplateErr)
	}

	// template must have validated
	log.Infof("%s\n", au.BrightGreen("✓"))
	//log.Infof("%+v\n", validateTemplateOutput.String())

	// look to see if stack exists
	log.Debug("Describing", stack.Name)
	describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}
	_, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)

	createChangeSetInput := cloudformation.CreateChangeSetInput{
		Capabilities:  validateTemplateOutput.Capabilities,
		ChangeSetName: aws.String(changeSetName), // I think AWS overuses pointers
		StackName:     aws.String(stack.Name),
		TemplateBody:  aws.String(templateBody),
	}
	changeSetType := "UPDATE" // default

	// if stack does not exist set action to CREATE
	if describeStacksErr != nil {
		changeSetType = "CREATE" // if stack does not already exist
	}
	createChangeSetInput.ChangeSetType = &changeSetType

	parametersMap := make(map[string]string)

	if !flags.DeployPrevious {
		// look for secrets file
		secretsPath := filepath.Clean(buildInstance.DisplayPath + "/secrets.env")
		if _, err := os.Stat(secretsPath); !os.IsNotExist(err) {
			if !flags.DeployPrevious {
				log.Infof("%s", au.Gray(11, "  Decrypting secrets..."))
			}

			secrets, secretsErr := stx.DecryptSecrets(secretsPath, stack.SopsProfile)

			if secretsErr != nil {
				log.Error(secretsErr)
				return
			}

			for k, v := range secrets {
				parametersMap[k] = v
			}

			log.Check()

			if flags.DeploySave {
				saveErr := saveStackOutputs(buildInstance, stack)
				if saveErr != nil {
					log.Error(saveErr)
				}
			}
		}

		paramsPath := filepath.Clean(buildInstance.DisplayPath + "/params.env")
		if _, err := os.Stat(paramsPath); !os.IsNotExist(err) {

			log.Infof("%s", au.Gray(11, "  Loading parameters..."))

			myEnv, err := godotenv.Read(paramsPath)

			if err != nil {
				log.Error(err)
				return
			}

			for k, v := range myEnv {
				parametersMap[k] = v
			}

			log.Check()

			if flags.DeploySave {
				saveErr := saveStackOutputs(buildInstance, stack)
				if saveErr != nil {
					log.Error(saveErr)
				}
			}
		}
	} else {
		// deploy using previous values
		stackParameters, _ := stackValue.Lookup("Template", "Parameters").Fields()
		for stackParameters.Next() {
			stackParam := stackParameters.Value()
			key, _ := stackParam.Label()
			parametersMap[key] = ""
		}
	}

	var parameters []*cloudformation.Parameter

	if flags.DeployPrevious {
		log.Infof("%s", au.Gray(11, "  Using previous parameters..."))
		log.Check()
	}

	for paramKey, paramVal := range parametersMap {
		myKey := paramKey
		myValue := paramVal
		parameter := cloudformation.Parameter{ParameterKey: aws.String(myKey)}

		if flags.DeployPrevious {
			parameter.SetUsePreviousValue(true)
		} else {
			parameter.ParameterValue = aws.String(myValue)
		}

		parameters = append(parameters, &parameter)
	}

	createChangeSetInput.SetParameters(parameters)

	// handle Stack.Tags
	if len(stack.Tags) > 0 {
		var tags []*cloudformation.Tag
		for k, v := range stack.Tags {
			tagK := k // reassign here to avoid issues with for-scope var
			var tagV string
			switch v {
			default:
				tagV = v
			case "${STX::CuePath}":
				tagV = strings.Replace(buildInstance.Dir, buildInstance.Root, "", 1)
			case "${STX::CueFiles}":
				tagV = strings.Join(buildInstance.CUEFiles, ", ")
			}
			tags = append(tags, &cloudformation.Tag{Key: &tagK, Value: &tagV})
		}
		createChangeSetInput.SetTags(tags)
	}

	log.Infof("%s", au.Gray(11, "  Creating changeset..."))

	_, createChangeSetErr := cfn.CreateChangeSet(&createChangeSetInput)

	if createChangeSetErr != nil {
		log.Fatal(createChangeSetErr)
	}

	describeChangesetInput := cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stack.Name),
	}

	waitOption := request.WithWaiterDelay(request.ConstantWaiterDelay(5 * time.Second))
	cfn.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), &describeChangesetInput, waitOption)

	log.Check()

	describeChangesetOuput, describeChangesetErr := cfn.DescribeChangeSet(&describeChangesetInput)
	if describeChangesetErr != nil {
		log.Fatalf("%+v", au.Red(describeChangesetErr))
	}

	if aws.StringValue(describeChangesetOuput.ExecutionStatus) != "AVAILABLE" || aws.StringValue(describeChangesetOuput.Status) != "CREATE_COMPLETE" {
		//TODO put describeChangesetOuput into table view
		log.Infof("%+v", describeChangesetOuput)
		log.Info("No changes to deploy.")
		return
	}

	if len(describeChangesetOuput.Changes) > 0 {
		log.Infof("%+v\n", describeChangesetOuput.Changes)
	}

	diff(cfn, stack.Name, templateBody)

	log.Infof("%s %s\n▶︎", au.BrightBlue("Execute change set?"), "Y to execute. Anything else to cancel.")
	var input string
	fmt.Scanln(&input)

	input = strings.ToLower(input)
	matched, _ := regexp.MatchString("^(y){1}(es)?$", input)
	if !matched {
		os.Exit(0) // exit if any other key were pressed
	}

	executeChangeSetInput := cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stack.Name),
	}

	log.Infof("%s %s %s %s:%s\n", au.White("Executing"), au.BrightBlue(changeSetName), au.White("⤏"), au.Magenta(stack.Name), au.Cyan(stack.Region))

	_, executeChangeSetErr := cfn.ExecuteChangeSet(&executeChangeSetInput)

	if executeChangeSetErr != nil {
		log.Fatal(executeChangeSetErr)
	}

	if flags.DeploySave || flags.DeployWait {
		log.Infof("%s", au.Gray(11, "  Waiting for stack..."))
		switch changeSetType {
		case "UPDATE":
			cfn.WaitUntilStackUpdateCompleteWithContext(context.Background(), &describeStacksInput, waitOption)
		case "CREATE":
			cfn.WaitUntilStackCreateCompleteWithContext(context.Background(), &describeStacksInput, waitOption)
		}
		log.Check()

		if flags.DeploySave {
			saveErr := saveStackOutputs(buildInstance, stack)
			if saveErr != nil {
				log.Fatal(saveErr)
			}
		}
	}
}
