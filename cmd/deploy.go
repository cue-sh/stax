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
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploys a stack by creating a changeset, previews expected changes, and optionally executes.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks, stacksErr := stx.GetStacks(cueValue, flags)
			if stacksErr != nil {
				log.Error(stacksErr)
			}

			if stacks == nil {
				return
			}

			for stackName, stack := range stacks {

				fileName, saveErr := saveStackAsYml(stackName, stack, buildInstance, cueValue)
				if saveErr != nil {
					log.Error(saveErr)
				}
				log.Infof("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stackName), au.White("⤏"), au.Green(stack.Profile), au.Cyan(stack.Region))
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
				log.Debug("Describing", stackName)
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
				_, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)

				createChangeSetInput := cloudformation.CreateChangeSetInput{
					Capabilities:  validateTemplateOutput.Capabilities,
					ChangeSetName: &changeSetName, // I think AWS overuses pointers
					StackName:     &stackName,
					TemplateBody:  &templateBody,
				}
				changeSetType := "UPDATE" // default

				// if stack does not exist set action to CREATE
				if describeStacksErr != nil {
					changeSetType = "CREATE" // if stack does not already exist
				}
				createChangeSetInput.ChangeSetType = &changeSetType

				parametersMap := make(map[string]string)
				// look for secrets file
				secretsPath := filepath.Clean(buildInstance.DisplayPath + "/secrets.env")
				if _, err := os.Stat(secretsPath); !os.IsNotExist(err) {
					log.Infof("%s", au.Gray(11, "  Decrypting secrets..."))

					secrets, secretsErr := stx.DecryptSecrets(secretsPath, stack.SopsProfile)

					if secretsErr != nil {
						log.Error(secretsErr)
						continue
					}
					for k, v := range secrets {
						parametersMap[k] = v
					}

					log.Infof("%s\n", au.Green("✓"))
				}

				paramsPath := filepath.Clean(buildInstance.DisplayPath + "/params.env")
				if _, err := os.Stat(paramsPath); !os.IsNotExist(err) {
					log.Infof("%s", au.Gray(11, "  Loading params..."))

					myEnv, err := godotenv.Read(paramsPath)

					if err != nil {
						log.Error(err)
						continue
					}
					for k, v := range myEnv {
						parametersMap[k] = v
					}

					log.Infof("%s\n", au.Green("✓"))
				}

				var parameters []*cloudformation.Parameter

				for paramKey, paramVal := range parametersMap {
					myKey := paramKey
					myValue := paramVal
					parameters = append(parameters, &cloudformation.Parameter{ParameterKey: &myKey, ParameterValue: &myValue})
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
				// s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Build our new spinner
				// s.Color("green")
				// s.Start()

				_, createChangeSetErr := cfn.CreateChangeSet(&createChangeSetInput)

				if createChangeSetErr != nil {
					log.Fatal(createChangeSetErr)
				}

				describeChangesetInput := cloudformation.DescribeChangeSetInput{
					ChangeSetName: &changeSetName,
					StackName:     &stackName,
				}

				cfn.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), &describeChangesetInput, request.WithWaiterDelay(request.ConstantWaiterDelay(5*time.Second)))
				// s.Stop()
				log.Infof("%s\n", au.Green("✓"))
				describeChangesetOuput, describeChangesetErr := cfn.DescribeChangeSet(&describeChangesetInput)
				if describeChangesetErr != nil {
					log.Fatalf("%+v", au.Red(describeChangesetErr))
				}

				if aws.StringValue(describeChangesetOuput.ExecutionStatus) != "AVAILABLE" || aws.StringValue(describeChangesetOuput.Status) != "CREATE_COMPLETE" {
					//TODO put describeChangesetOuput into table view
					log.Infof("%+v", describeChangesetOuput)
					log.Info("No changes to deploy.")
					continue
				}

				if len(describeChangesetOuput.Changes) > 0 {
					log.Infof("%+v\n", describeChangesetOuput.Changes)
					diff(cfn, stackName, templateBody)
				} else {
					log.Info("No changes to resources.")
					continue
				}

				log.Infof("%s %s\n▶︎", au.BrightBlue("Execute change set?"), "Y to execute. Anything else to cancel.")
				var input string
				fmt.Scanln(&input)

				input = strings.ToLower(input)
				matched, _ := regexp.MatchString("^(y){1}(es)?$", input)
				if !matched {
					os.Exit(0) // exit if any other key were pressed
				}

				executeChangeSetInput := cloudformation.ExecuteChangeSetInput{
					ChangeSetName: &changeSetName,
					StackName:     &stackName,
				}

				log.Infof("%s %s %s %s:%s\n", au.White("Executing"), au.BrightBlue(changeSetName), au.White("⤏"), au.Magenta(stackName), au.Cyan(stack.Region))

				_, executeChangeSetErr := cfn.ExecuteChangeSet(&executeChangeSetInput)

				if executeChangeSetErr != nil {
					log.Fatal(executeChangeSetErr)
				}

			}

		})
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
