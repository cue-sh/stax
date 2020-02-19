package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"github.com/spf13/cobra"
)

// exeCmd represents the exe command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "DIFF against CloudFormation for the evaluted leaves.",
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

					// get a session and cloudformation service client
					session := stx.GetSession(stack.Profile)
					cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

					// read template from disk
					templateFileBytes, _ := ioutil.ReadFile(fileName)
					templateBody := string(templateFileBytes)

					// look to see if stack exists
					describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
					describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)

					if describeStacksErr != nil {
						fmt.Printf("DESC STAX:\n%+v %+v", describeStacksOutput, describeStacksErr)
						continue
					}

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

				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// TODO add a flag to watch events
}
