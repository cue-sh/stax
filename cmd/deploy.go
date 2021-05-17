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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/cue-sh/stax/graph"
	"github.com/cue-sh/stax/internal"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&flags.DeployWait, "wait", "w", false, "Wait for stack updates to complete before continuing.")
	deployCmd.Flags().BoolVarP(&flags.DeploySave, "save", "s", false, "Save stack outputs upon successful completion. Implies --wait.")
	deployCmd.Flags().BoolVarP(&flags.DeployDeps, "dependencies", "d", false, "Deploy stack dependencies in order. Implies --save.")
	deployCmd.Flags().BoolVarP(&flags.DeployPrevious, "previous-values", "v", false, "Deploy stack using previous parameter values.")
}

type deployArgs struct {
	stack         internal.Stack
	stackValue    cue.Value
	buildInstance *build.Instance
}

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys a stack by creating a changeset, previews expected changes, and optionally executes.",
	Long: `Deploy will act upon every stack it finds among the evaluated cue files.
	
For each stack, a changeset is first created, and the proposed changes are
displayed. At this point you have the option to execute the changeset
before moving on to the next stack.

The following config.internal.cue options are available:

Cmd: {
  Deploy: {
    Notify: {
      Endpoint: string | *""
      TopicArn: string | *""
    }
  }
}

Use Cmd:Deploy:Notify: properties to enable the notify command to receive stack
event notifications from SNS. The endpoint will be the http address provided by
the notify command. If this is run behind a router, you will need to enable
port forwarding. If port forwarding is not possible, such as in a corporate
office setting, stax notify could be run on a remote machine such as an EC2 
instance, or virtual workspace.

The TopicArn is an SNS topic that is provided as a NotificationArn when
creating changesets. In a team setting, it may be better for each member to
have their own topic; keep in mind that the last person to deploy will be
the one to receive notifications when the stack is deleted. To receive events
during a delete operation, be sure to update the stack with your own TopicArn
first.

`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()
		internal.EnsureVaultSession(config)

		if flags.DeployDeps {
			flags.DeploySave = true
		}

		availableStacks := make(map[string]deployArgs)
		workingGraph := graph.NewGraph()
		buildInstances := internal.GetBuildInstances(args, config.PackageName)

		internal.Process(config, buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := internal.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			// since the Process handler generally only sees one stack per instance,
			// we need to gather ALL the stacks first primarily to support dependencies
			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack internal.Stack
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

func deployStack(stack internal.Stack, buildInstance *build.Instance, stackValue cue.Value) {

	fileName, saveErr := saveStackAsYml(stack, buildInstance, stackValue)
	if saveErr != nil {
		log.Error(saveErr)
	}
	log.Infof("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stack.Name), au.White("⤏"), au.Green(stack.Profile), au.Cyan(stack.Region))
	log.Infof("%s", au.Gray(11, "  Validating template..."))

	// get a session and cloudformation service client
	log.Debugf("\nGetting session for profile %s\n", stack.Profile)
	session := internal.GetSession(stack.Profile)
	awsCfg := aws.NewConfig().WithRegion(stack.Region)
	cfn := cloudformation.New(session, awsCfg)

	// read template from disk
	log.Debug("Reading template from", fileName)
	templateFileBytes, _ := ioutil.ReadFile(fileName)
	templateBody := string(templateFileBytes)
	usr, _ := user.Current()

	changeSetName := "stax-dpl-" + usr.Username + "-" + fmt.Sprintf("%x", sha1.Sum(templateFileBytes))
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

	stackParametersValue := stackValue.Lookup("Template", "Parameters")
	if stackParametersValue.Exists() {

		// TODO paramaters need to support the type as declared in Parameter.Type (in the least string and number).
		// this should be map[string]interface{} with type casting done when adding parameters to the changeset
		parametersMap := make(map[string]string)
		var parameters []*cloudformation.Parameter

		if flags.DeployPrevious {
			// deploy using previous values
			stackParameters, stackParametersErr := stackParametersValue.Fields()
			if stackParametersErr != nil {
				log.Fatal(stackParametersErr)
				return
			}
			log.Infof("%s", au.Gray(11, "  Using previous parameters..."))
			for stackParameters.Next() {
				stackParam := stackParameters.Value()
				key, _ := stackParam.Label()
				parametersMap[key] = ""
			}
			log.Check()
		} else {
			// load overrides

			// TODO #48 stax should prompt for each Parameter input if overrides are undefined
			if len(stack.Overrides) < 0 {
				log.Fatal("Template has Parameters but no Overrides are defined.")
				return
			}

			for k, v := range stack.Overrides {
				path := strings.Replace(k, "${STX::CuePath}", strings.Replace(buildInstance.Dir, buildInstance.Root+"/", "", 1), 1)
				behavior := v

				log.Infof("%s", au.Gray(11, "  Applying overrides: "+path+" "))

				var yamlBytes []byte
				var yamlBytesErr error

				if behavior.SopsProfile != "" {
					// decrypt the file contents
					yamlBytes, yamlBytesErr = internal.DecryptSecrets(filepath.Clean(buildInstance.Root+"/"+path), behavior.SopsProfile)
				} else {
					// just pull the file contents directly
					yamlBytes, yamlBytesErr = ioutil.ReadFile(filepath.Clean(buildInstance.Root + "/" + path))
				}

				if yamlBytesErr != nil {
					log.Fatal(yamlBytesErr)
					return
				}

				// TODO #47 parameters need to support the type as declared in Parameter.Type (in the least string and number).
				// this should be map[string]interface{} with type casting done when adding parameters to the changeset
				var override map[string]string

				yamlUnmarshalErr := yaml.Unmarshal(yamlBytes, &override)
				if yamlUnmarshalErr != nil {
					log.Fatal(yamlUnmarshalErr)
					return
				}

				// TODO #50 stax should error when a parameter key is duplicated among two or more overrides files
				if len(behavior.Map) > 0 {
					// map the yaml key:value to parameter key:value
					for k, v := range behavior.Map {
						fromKey := k
						toKey := v
						parametersMap[toKey] = override[fromKey]
					}
				} else {
					// just do a straight copy, keys should align 1:1
					for k, v := range override {
						overrideKey := k
						overrideVal := v
						parametersMap[overrideKey] = overrideVal
					}
				}
				log.Check()
			}
		}

		// apply parameters to changeset
		for k, v := range parametersMap {
			paramKey := k
			paramVal := v
			parameter := cloudformation.Parameter{ParameterKey: aws.String(paramKey)}

			if flags.DeployPrevious {
				parameter.SetUsePreviousValue(true)
			} else {
				parameter.ParameterValue = aws.String(paramVal)
			}

			parameters = append(parameters, &parameter)
		}

		createChangeSetInput.SetParameters(parameters)

	} // end stackParametersValue.Exists()

	// handle Stack.Tags
	if len(stack.Tags) > 0 && stack.TagsEnabled {
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
	if config.Cmd.Deploy.Notify.TopicArn != "" { // && stax notify command is running! perhaps use unix domain sockets to test
		log.Infof("%s", au.Gray(11, "  Reticulating splines..."))

		snsClient := sns.New(session, awsCfg)

		subscribeInput := sns.SubscribeInput{Endpoint: aws.String(config.Cmd.Deploy.Notify.Endpoint), TopicArn: aws.String(config.Cmd.Deploy.Notify.TopicArn), Protocol: aws.String("http")}
		_, subscribeErr := snsClient.Subscribe(&subscribeInput)
		if subscribeErr != nil {
			log.Errorf("%s\n", subscribeErr)
		} else {
			var notificationArns []*string
			notificationArns = append(notificationArns, aws.String(config.Cmd.Deploy.Notify.TopicArn))
			createChangeSetInput.SetNotificationARNs(notificationArns)
			log.Check()
		}
	}

	log.Infof("%s", au.Gray(11, "  Creating changeset..."))

	_, createChangeSetErr := cfn.CreateChangeSet(&createChangeSetInput)

	if createChangeSetErr != nil {
		if awsErr, ok := createChangeSetErr.(awserr.Error); ok {
			log.Infof(" %s\n", au.Red(awsErr))
			if awsErr.Code() == "AlreadyExistsException" {
				var deleteChangesetInput cloudformation.DeleteChangeSetInput
				deleteChangesetInput.ChangeSetName = createChangeSetInput.ChangeSetName
				deleteChangesetInput.StackName = createChangeSetInput.StackName
				log.Infof("%s %s\n", au.White("Deleting"), au.BrightBlue(changeSetName))
				_, deleteChangeSetErr := cfn.DeleteChangeSet(&deleteChangesetInput)
				if deleteChangeSetErr != nil {
					log.Error(deleteChangeSetErr)
				}
				return
			}
		}
		log.Fatal(createChangeSetErr)
	}

	describeChangesetInput := cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stack.Name),
	}

	waitOption := request.WithWaiterDelay(request.ConstantWaiterDelay(5 * time.Second))
	cfn.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), &describeChangesetInput, waitOption)

	log.Check()

	log.Infof("%s %s %s %s:%s\n", au.White("Describing"), au.BrightBlue(changeSetName), au.White("⤎"), au.Magenta(stack.Name), au.Cyan(stack.Region))
	describeChangesetOuput, describeChangesetErr := cfn.DescribeChangeSet(&describeChangesetInput)
	if describeChangesetErr != nil {
		log.Fatalf("%+v", au.Red(describeChangesetErr))
	}

	if aws.StringValue(describeChangesetOuput.ExecutionStatus) != "AVAILABLE" || aws.StringValue(describeChangesetOuput.Status) != "CREATE_COMPLETE" {
		//TODO put describeChangesetOuput into table view
		log.Infof("%+v\n", describeChangesetOuput)
		log.Info(au.Yellow("No changes to deploy."))
		var deleteChangesetInput cloudformation.DeleteChangeSetInput
		deleteChangesetInput.ChangeSetName = createChangeSetInput.ChangeSetName
		deleteChangesetInput.StackName = createChangeSetInput.StackName
		log.Infof("%s %s\n", au.White("Deleting"), au.BrightBlue(changeSetName))
		_, deleteChangeSetErr := cfn.DeleteChangeSet(&deleteChangesetInput)
		if deleteChangeSetErr != nil {
			log.Error(deleteChangeSetErr)
		}
		return
	}

	if len(describeChangesetOuput.Changes) > 0 {
		// log.Infof("%+v\n", describeChangesetOuput.Changes)
		table := tablewriter.NewWriter(os.Stdout)
		table.SetAutoWrapText(false)
		table.SetAutoMergeCells(true)
		table.SetRowLine(true)
		table.SetHeader([]string{"Resource", "Action", "Attribute", "Property", "Recreation"})
		table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})

		for _, change := range describeChangesetOuput.Changes {

			row := []string{
				aws.StringValue(change.ResourceChange.LogicalResourceId),
				aws.StringValue(change.ResourceChange.Action),
				"",
				"",
				"",
			}

			if aws.StringValue(change.ResourceChange.Action) == "Modify" {
				for _, detail := range change.ResourceChange.Details {
					row[2] = aws.StringValue(detail.Target.Attribute)
					row[3] = aws.StringValue(detail.Target.Name)
					recreation := aws.StringValue(detail.Target.RequiresRecreation)

					if recreation == "ALWAYS" || recreation == "CONDITIONAL" {
						row[4] = au.Red(recreation).String()
					} else {
						row[4] = recreation
					}
					table.Append(row)
				}
			} else {
				table.Append(row)
			}
		}
		table.Render()
	}

	diff(cfn, stack.Name, templateBody)

	log.Infof("%s %s %s %s %s:%s:%s %s\n", au.Index(255-88, "Execute change set"), au.BrightBlue(changeSetName), au.Index(255-88, "on"), au.White("⤏"), au.Magenta(stack.Name), au.Green(stack.Profile), au.Cyan(stack.Region), au.Index(255-88, "?"))
	log.Infof("%s\n%s", au.Gray(11, "Y to execute. Anything else to cancel."), au.Gray(11, "▶︎"))
	var input string
	fmt.Scanln(&input)

	input = strings.ToLower(input)
	matched, _ := regexp.MatchString("^(y){1}(es)?$", input)
	if !matched {
		// delete changeset and continue
		var deleteChangesetInput cloudformation.DeleteChangeSetInput
		deleteChangesetInput.ChangeSetName = createChangeSetInput.ChangeSetName
		deleteChangesetInput.StackName = createChangeSetInput.StackName
		log.Infof("%s %s\n", au.White("Deleting"), au.BrightBlue(changeSetName))
		_, deleteChangeSetErr := cfn.DeleteChangeSet(&deleteChangesetInput)
		if deleteChangeSetErr != nil {
			log.Error(deleteChangeSetErr)
		}
		return
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
			saveErr := saveStackOutputs(config, buildInstance, stack)
			if saveErr != nil {
				log.Fatal(saveErr)
			}
		}
	}
}
