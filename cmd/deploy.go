package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	cueYaml "cuelang.org/go/pkg/encoding/yaml"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/cue-sh/stax/graph"
	"github.com/cue-sh/stax/internal"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	goYaml "gopkg.in/yaml.v2"
)

func init() {
	rootCmd.AddCommand(deployCmd)
	deployCmd.Flags().BoolVarP(&flags.DeployWait, "wait", "w", false, "Wait for stack updates to complete before continuing.")
	deployCmd.Flags().BoolVarP(&flags.DeploySave, "save", "s", false, "Save stack outputs upon successful completion. Implies --wait.")
	deployCmd.Flags().BoolVarP(&flags.DeployDeps, "dependencies", "d", false, "Deploy stack dependencies in order. Implies --save.")
	deployCmd.Flags().BoolVarP(&flags.DeployPrevious, "previous-values", "v", false, "Deploy stack using previous parameter values.")
	deployCmd.Flags().BoolVar(&flags.DeployNoExecute, "no-execute", false, "Creates the change set only.")
	// deployCmd.Flags().BoolVar(&flags.DeployYesExecute, "yes-execute", false, "Creates the change set and immediately executes without prompting.")
	deployCmd.Flags().BoolVar(&flags.DeployExecuteOnly, "execute-only", false, "Executes previously created changesets.")
}

func getChangeSetName(stack internal.Stack, stackValue cue.Value) (string, error) {
	stackHash, stackHashErr := internal.GetStackHash(stack, stackValue)
	if stackHashErr != nil {
		return "", stackHashErr
	}
	return "stax-" + stackHash, nil
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
`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		if flags.DeployExecuteOnly && flags.DeployNoExecute {
			log.Fatal("Cannot set both --no-execute and --execute-only")
			return
		}

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

	log.Infof("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stack.Name), au.White("⤏"), au.Green(stack.Profile), au.Cyan(stack.Region))
	log.Debug("Getting change set name")
	changeSetName, changeSetNameErr := getChangeSetName(stack, stackValue)
	if changeSetNameErr != nil {
		log.Error(changeSetNameErr)
		return
	}

	// get a session and cloudformation service client
	log.Debugf("\nGetting session for %s:%s\n", stack.Profile, stack.Region)
	// get a session and cloudformation service client
	cfn := internal.GetCloudFormationClient(stack.Profile, stack.Region)

	describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}

	if !flags.DeployExecuteOnly {
		log.Infof("%s", au.Gray(11, "  Validating template..."))

		template := stackValue.Lookup("Template")
		templateBody, ymlErr := cueYaml.Marshal(template)
		if ymlErr != nil {
			log.Error(ymlErr)
			return
		}

		// validate template
		validateTemplateInput := &cloudformation.ValidateTemplateInput{
			TemplateBody: aws.String(templateBody),
		}
		validateTemplateOutput, validateTemplateErr := cfn.ValidateTemplate(context.TODO(), validateTemplateInput)

		// template failed to validate
		if validateTemplateErr != nil {
			log.X()
			log.Fatalf("%+v\n", validateTemplateErr)
		}

		// template must have validated
		log.Check()

		// look to see if stack exists
		log.Debug("  Describing", stack.Name)
		_, describeStacksErr := cfn.DescribeStacks(context.TODO(), &describeStacksInput)

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

		createChangeSetInput.ChangeSetType = types.ChangeSetType(changeSetType)

		stackParametersValue := stackValue.Lookup("Template", "Parameters")
		if stackParametersValue.Exists() {

			// TODO paramaters need to support the type as declared in Parameter.Type (in the least string and number).
			// this should be map[string]interface{} with type casting done when adding parameters to the changeset
			parametersMap := make(map[string]string)
			var parameters []types.Parameter

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
				if len(stack.Overrides) < 1 {
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

					yamlUnmarshalErr := goYaml.Unmarshal(yamlBytes, &override)
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
				parameter := types.Parameter{ParameterKey: aws.String(paramKey)}

				if flags.DeployPrevious {
					parameter.UsePreviousValue = aws.Bool(true)
				} else {
					parameter.ParameterValue = aws.String(paramVal)
				}

				parameters = append(parameters, parameter)
			}

			createChangeSetInput.Parameters = parameters

		} // end stackParametersValue.Exists()

		// handle Stack.Tags
		if len(stack.Tags) > 0 && stack.TagsEnabled {
			var tags []types.Tag
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
				tags = append(tags, types.Tag{Key: &tagK, Value: &tagV})
			}
			createChangeSetInput.Tags = tags
		}

		if stack.Role != "" {
			log.Infof("%s", au.Gray(11, "  Applying role: "+stack.Role+"\n"))
			createChangeSetInput.RoleARN = aws.String(stack.Role)
		}

		_, createChangeSetErr := cfn.CreateChangeSet(context.TODO(), &createChangeSetInput)

		if createChangeSetErr != nil {
			log.Infof(" %s\n", au.Red(createChangeSetErr))
			if createChangeSetErr.Error() == "AlreadyExistsException" {
				var deleteChangesetInput cloudformation.DeleteChangeSetInput
				deleteChangesetInput.ChangeSetName = createChangeSetInput.ChangeSetName
				deleteChangesetInput.StackName = createChangeSetInput.StackName
				log.Infof("%s %s\n", au.White("Deleting"), au.BrightBlue(changeSetName))
				_, deleteChangeSetErr := cfn.DeleteChangeSet(context.TODO(), &deleteChangesetInput)
				if deleteChangeSetErr != nil {
					log.Error(deleteChangeSetErr)
				}
				return
			}
		}

		log.Infof("%s %s", au.Gray(11, "  Awaiting changeset"), au.BrightBlue(changeSetName))

		describeChangesetInput := cloudformation.DescribeChangeSetInput{
			ChangeSetName: aws.String(changeSetName),
			StackName:     aws.String(stack.Name),
		}

		var describeChangesetOuput *cloudformation.DescribeChangeSetOutput
		var describeChangesetErr error

		// TODO: make this a waiter when v2 finally supports them
		// waits for 10m polling every 5s
		for i := 0; i < 120; i++ {
			describeChangesetOuput, describeChangesetErr = cfn.DescribeChangeSet(context.TODO(), &describeChangesetInput)
			if describeChangesetErr != nil {
				log.X()
				log.Fatalf("%+v", au.Red(describeChangesetErr))
				break
			}

			if describeChangesetOuput.Status != types.ChangeSetStatusCreateInProgress && describeChangesetOuput.Status != types.ChangeSetStatusCreatePending {
				break
			}

			log.Debugf("Changeset is %s. Polling again in 5s.\n", describeChangesetOuput.Status)
			time.Sleep(5 * time.Second)
		}

		log.Check()

		log.Infof("%s %s %s %s:%s:%s\n", au.White("Describing"), au.BrightBlue(changeSetName), au.White("⤎"), au.Magenta(stack.Name), au.Green(stack.Profile), au.Cyan(stack.Region))

		if describeChangesetOuput.ExecutionStatus != types.ExecutionStatusAvailable || describeChangesetOuput.Status != types.ChangeSetStatusCreateComplete {
			//TODO put describeChangesetOuput into table view
			log.Debugf("%+v\n", describeChangesetOuput)
			log.Info(au.Yellow("No changes to deploy."))

			var deleteChangesetInput cloudformation.DeleteChangeSetInput
			deleteChangesetInput.ChangeSetName = createChangeSetInput.ChangeSetName
			deleteChangesetInput.StackName = createChangeSetInput.StackName
			log.Infof("%s %s\n", au.White("Deleting"), au.BrightBlue(changeSetName))

			_, deleteChangeSetErr := cfn.DeleteChangeSet(context.TODO(), &deleteChangesetInput)
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
					aws.ToString(change.ResourceChange.LogicalResourceId),
					string(change.ResourceChange.Action),
					"",
					"",
					"",
				}

				if change.ResourceChange.Action == types.ChangeActionModify {
					for _, detail := range change.ResourceChange.Details {
						row[2] = string(detail.Target.Attribute)
						row[3] = aws.ToString(detail.Target.Name)
						recreation := detail.Target.RequiresRecreation

						if recreation == types.RequiresRecreationAlways || recreation == types.RequiresRecreationConditionally {
							row[4] = au.Red(recreation).String()
						} else {
							row[4] = string(recreation)
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

		if flags.DeployNoExecute {
			return
		}

		log.Infof("%s %s %s %s:%s:%s %s\n", au.Index(255-88, "Execute change set"), au.BrightBlue(changeSetName), au.White("⤏"), au.Magenta(stack.Name), au.Green(stack.Profile), au.Cyan(stack.Region), au.Index(255-88, "?"))
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
			_, deleteChangeSetErr := cfn.DeleteChangeSet(context.TODO(), &deleteChangesetInput)
			if deleteChangeSetErr != nil {
				log.Error(deleteChangeSetErr)
			}
			return
		}
	} // end if !flags.DeployExecuteOnly

	executeChangeSetInput := cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stack.Name),
	}

	log.Infof("%s %s %s %s:%s:%s\n", au.White("Executing"), au.BrightBlue(changeSetName), au.White("⤏"), au.Magenta(stack.Name), au.Green(stack.Profile), au.Cyan(stack.Region))

	_, executeChangeSetErr := cfn.ExecuteChangeSet(context.TODO(), &executeChangeSetInput)

	if executeChangeSetErr != nil {
		log.Fatal(executeChangeSetErr)
	}

	if flags.DeploySave || flags.DeployWait {
		log.Infof("%s", au.Gray(11, "  Waiting for stack..."))

		var describeStacksOuput *cloudformation.DescribeStacksOutput
		var describeStacksErr error
		var stackStatus types.StackStatus

		// TODO make this a waiter when v2 supports them
		// wait for 1h polling every 5s
		for i := 0; i < 720; i++ {
			describeStacksOuput, describeStacksErr = cfn.DescribeStacks(context.TODO(), &describeStacksInput)
			if describeStacksErr != nil {
				log.Fatalf("%+v", au.Red(describeStacksErr))
				break
			}

			stackStatus = describeStacksOuput.Stacks[0].StackStatus
			if stackStatus != types.StackStatusCreateInProgress &&
				stackStatus != types.StackStatusUpdateInProgress &&
				stackStatus != types.StackStatusUpdateCompleteCleanupInProgress &&
				stackStatus != types.StackStatusImportInProgress {
				break
			}

			log.Debugf("Stack is %s. Polling again in 5s.\n", stackStatus)
			time.Sleep(5 * time.Second)
		}

		if stackStatus != types.StackStatusCreateComplete && stackStatus != types.StackStatusUpdateComplete {
			log.Errorf("Stack failed with status %s", stackStatus)
		} else {
			log.Check()

			if flags.DeploySave {
				saveErr := saveStackOutputs(config, buildInstance, stack)
				if saveErr != nil {
					log.Fatal(saveErr)
				}
			}
		}
	}
}
