package cmd

import (
	"context"
	"io/ioutil"
	"os"
	"regexp"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/cue-sh/stax/internal"
	"github.com/gonvenience/ytbx"
	"github.com/homeport/dyff/pkg/dyff"
	"github.com/spf13/cobra"
)

// exeCmd represents the exe command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff against the current CloudFormation template.",
	Long: `Diff will operate upon every stack found among the evaluated cue files.
	
For each stack, diff will first export the stack, download the template stored
in CloudFormation, then produce a rich, functional, property-based diff (not 
text-based) against the two templates.

Diff is an implementation of https://github.com/homeport/dyff
`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		buildInstances := internal.GetBuildInstances(args, config.PackageName)

		internal.Process(config, buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := internal.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack internal.Stack
				decodeErr := stackValue.Decode(&stack)
				if decodeErr != nil {
					log.Error(decodeErr)
					continue
				}

				fileName, saveErr := saveStackAsYml(stack, buildInstance, stackValue)
				if saveErr != nil {
					log.Error(saveErr)
				}

				// get a session and cloudformation service client
				cfn := internal.GetCloudFormationClient(stack.Profile, stack.Region)

				// read template from disk
				templateFileBytes, _ := ioutil.ReadFile(fileName)
				templateBody := string(templateFileBytes)

				// look to see if stack exists
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}
				describeStacksOutput, describeStacksErr := cfn.DescribeStacks(context.TODO(), &describeStacksInput)

				if describeStacksErr != nil {
					log.Debugf("DESC STAX:\n%+v\n", describeStacksOutput)
					log.Error(describeStacksErr)
					continue
				}

				diff(cfn, stack.Name, templateBody)
			}

		})
	},
}

func diff(cfn *cloudformation.Client, stackName, templateBody string) {
	existingTemplate, err := cfn.GetTemplate(context.TODO(), &cloudformation.GetTemplateInput{
		StackName: &stackName,
	})
	if err != nil {
		log.Error("Error getting template for stack", stackName)
	} else {
		r, _ := regexp.Compile("!(Base64|Cidr|FindInMap|GetAtt|GetAZs|ImportValue|Join|Select|Split|Sub|Transform|Ref|And|Equals|If|Not|Or)")
		if r.MatchString(aws.ToString(existingTemplate.TemplateBody)) {
			log.Warn("The existing stack uses short intrinsic functions, unable to create diff: " + stackName)
		} else {
			existingDoc, _ := ytbx.LoadDocuments([]byte(aws.ToString(existingTemplate.TemplateBody)))
			doc, _ := ytbx.LoadDocuments([]byte(templateBody))
			report, err := dyff.CompareInputFiles(
				ytbx.InputFile{Documents: existingDoc},
				ytbx.InputFile{Documents: doc},
			)
			if err != nil {
				log.Error("Error creating template diff for stack: " + stackName)
			} else {
				if len(report.Diffs) > 0 {
					reportWriter := &dyff.HumanReport{
						Report:     report,
						ShowBanner: false,
					}
					reportWriter.WriteReport(os.Stdout)
				}
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// TODO add a flag to watch events
}
