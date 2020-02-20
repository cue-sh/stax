package cmd

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"github.com/spf13/cobra"
)

// dplCmd represents the dpl command
var dplCmd = &cobra.Command{
	Use:   "dpl",
	Short: "DePLoys a stack by creating a changeset and previews expected changes.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {
		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags.exclude, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue)
			if stacks != nil {
				//fmt.Printf("%+v\n\n", top)

				for stackName, stack := range stacks {

					fileName := saveStackAsYml(stackName, stack, buildInstance, cueValue)
					fmt.Printf("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stackName), au.White("⤏"), au.Green(stack.Profile), au.Cyan(stack.Region))
					fmt.Print(au.Gray(11, "  Validating template..."))

					// get a session and cloudformation service client
					session := stx.GetSession(stack.Profile)
					cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

					// read template from disk
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
						fmt.Printf(" %s\n", au.Red("✕"))
						fmt.Printf("%+v\n", validateTemplateErr)
						// os.Exit(1)
						continue
					}

					// template must have validated
					fmt.Printf("%s\n", au.BrightGreen("✓"))
					//fmt.Printf("%+v\n", validateTemplateOutput.String())

					// look to see if stack exists
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
						//fmt.Printf("DESC STAX:\n%+v %+v", describeStacksOutput, describeStacksErr)
					}
					createChangeSetInput.ChangeSetType = &changeSetType

					// look for secrets file
					secretsPath := filepath.Clean(buildInstance.DisplayPath + "/secrets.env")
					if _, err := os.Stat(secretsPath); !os.IsNotExist(err) {
						fmt.Print(au.Gray(11, "  Decrypting secrets..."))
						secrets := stx.DecryptSecrets(secretsPath, config.Sops.Profile)
						// TODO check error
						// sops output is key=value\n so first split on new line
						var parameters []*cloudformation.Parameter

						for sopsKey, sopsValue := range secrets {
							parameter := cloudformation.Parameter{ParameterKey: &sopsKey, ParameterValue: &sopsValue}
							parameters = append(parameters, &parameter)
						}
						createChangeSetInput.SetParameters(parameters)

						fmt.Printf("%s\n", au.Green("✓"))
					}
					fmt.Print(au.Gray(11, "  Creating changeset..."))
					// s := spinner.New(spinner.CharSets[14], 100*time.Millisecond) // Build our new spinner
					// s.Color("green")
					// s.Start()

					createChangeSetOutput, createChangeSetErr := cfn.CreateChangeSet(&createChangeSetInput)

					if createChangeSetErr != nil {
						//fmt.Printf("%+v %+v", describeStacksOutput, describeStacksErr)
						fmt.Printf("%+v %+v", createChangeSetOutput, createChangeSetErr)
						os.Exit(1)
					}
					fmt.Printf("%s\n", au.Green("✓"))
					describeChangesetInput := cloudformation.DescribeChangeSetInput{
						ChangeSetName: &changeSetName,
						StackName:     &stackName,
					}

					cfn.WaitUntilChangeSetCreateComplete(&describeChangesetInput)
					// s.Stop()
					describeChangesetOuput, describeChangesetErr := cfn.DescribeChangeSet(&describeChangesetInput)
					if describeChangesetErr != nil {
						fmt.Printf("%+v", au.Red(describeChangesetErr))
						// os.Exit(1)
						continue
					}

					if *describeChangesetOuput.ExecutionStatus != "AVAILABLE" || *describeChangesetOuput.Status != "CREATE_COMPLETE" {
						fmt.Printf("%+v", describeChangesetOuput)
						fmt.Println("No changes to deploy.")
						// os.Exit(0)
						continue
					}

					if len(describeChangesetOuput.Changes) > 0 {
						fmt.Printf("%+v\n", describeChangesetOuput.Changes)
						existingTemplate, err := cfn.GetTemplate(&cloudformation.GetTemplateInput{
							StackName: &stackName,
						})
						if err != nil {
							fmt.Printf("%+v\n", au.Red("Error getting template for stack: "+stackName))
							continue
						} else {
							existingDoc, _ := ytbx.LoadDocuments([]byte(*existingTemplate.TemplateBody))
							doc, _ := ytbx.LoadDocuments([]byte(templateBody))
							report, err := dyff.CompareInputFiles(
								ytbx.InputFile{Documents: existingDoc},
								ytbx.InputFile{Documents: doc},
							)
							if err != nil {
								fmt.Printf("%+v\n", au.Red("Error creating template diff for stack: "+stackName))
								continue
							} else {
								reportWriter := &dyff.HumanReport{
									Report:     report,
									ShowBanner: false,
								}
								reportWriter.WriteReport(os.Stdout)
							}
						}
					} else {
						fmt.Println("No changes to resources.")
						continue
					}

					fmt.Printf("%s %s\n▶︎", au.BrightBlue("Execute change set?"), "Y to execute. Anything else to cancel.")
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

					_, executeChangeSetErr := cfn.ExecuteChangeSet(&executeChangeSetInput)

					if executeChangeSetErr != nil {
						fmt.Printf("%+v", au.Red(executeChangeSetErr))
						// os.Exit(1)
						continue
					}

					fmt.Printf("%s %s %s %s:%s\n", au.White("Executing"), au.BrightBlue(changeSetName), au.White("⤏"), au.Magenta(stackName), au.Cyan(stack.Region))
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(dplCmd)
}
