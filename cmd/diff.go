package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

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
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue, flags)
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
						fmt.Printf("DESC STAX:\n%+v %+v", describeStacksOutput, au.Red(describeStacksErr))
						continue
					}

					diff(cfn, stackName, templateBody)
				}
			}
		})
	},
}

func diff(cfn *cloudformation.CloudFormation, stackName, templateBody string) {
	existingTemplate, err := cfn.GetTemplate(&cloudformation.GetTemplateInput{
		StackName: &stackName,
	})
	if err != nil {
		fmt.Printf("%+v\n", au.Red("Error getting template for stack: "+stackName))
	} else {
		// fmt.Println(*existingTemplate.TemplateBody)
		r, _ := regexp.Compile("!(Base64|Cidr|FindInMap|GetAtt|GetAZs|ImportValue|Join|Select|Split|Sub|Transform|Ref|And|Equals|If|Not|Or)")
		if r.MatchString(*existingTemplate.TemplateBody) {
			fmt.Printf("  %+v\n", au.Red("The existing stack uses short intrinsic functions, unable to create diff: "+stackName))
		} else {
			existingDoc, _ := ytbx.LoadDocuments([]byte(*existingTemplate.TemplateBody))
			doc, _ := ytbx.LoadDocuments([]byte(templateBody))
			report, err := dyff.CompareInputFiles(
				ytbx.InputFile{Documents: existingDoc},
				ytbx.InputFile{Documents: doc},
			)
			if err != nil {
				fmt.Printf("%+v\n", au.Red("Error creating template diff for stack: "+stackName))
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

func init() {
	rootCmd.AddCommand(diffCmd)

	// TODO add a flag to watch events
}
