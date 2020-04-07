package cmd

import (
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(resourcesCmd)
}

// resourcesCmd represents the resources command
var resourcesCmd = &cobra.Command{
	Use:   "resources",
	Short: "Lists the resources managed by the stack.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {

		// TODO add debug messages
		defer log.Flush()
		stx.EnsureVaultSession(config)

		buildInstances := stx.GetBuildInstances(args, "cfn")

		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := stx.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack stx.Stack
				decodeErr := stackValue.Decode(&stack)
				if decodeErr != nil {
					log.Error(decodeErr)
					continue
				}

				// get a session and cloudformation service client
				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))
				log.Infof("%s %s...\n", au.White("Describing"), au.Magenta(stack.Name))

				describeStackResourcesInput := cloudformation.DescribeStackResourcesInput{StackName: aws.String(stack.Name)}
				describeStackResourcesOutput, describeStackResourcesErr := cfn.DescribeStackResources(&describeStackResourcesInput)
				if describeStackResourcesErr != nil {
					log.Error(describeStackResourcesErr)
					continue
				}
				// TODO add --aws-output(?) to be used in conjunction with --debug
				// log.Debugf("%+v\n", describeStackResourcesOutput)

				table := tablewriter.NewWriter(os.Stdout)
				table.SetAutoWrapText(false)
				table.SetHeader([]string{"Logical ID", "Physical ID", "Type", "Status"})
				table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})

				for _, resource := range describeStackResourcesOutput.StackResources {

					status := aws.StringValue(resource.ResourceStatus)
					if strings.Contains(aws.StringValue(resource.ResourceStatus), "COMPLETE") {
						status = au.BrightGreen(aws.StringValue(resource.ResourceStatus)).String()
					}

					if strings.Contains(aws.StringValue(resource.ResourceStatus), "FAIL") || strings.Contains(aws.StringValue(resource.ResourceStatus), "ROLLBACK") {
						status = au.Red(aws.StringValue(resource.ResourceStatus)).String()
					}

					table.Append([]string{aws.StringValue(resource.LogicalResourceId), aws.StringValue(resource.PhysicalResourceId), aws.StringValue(resource.ResourceType), status})
				}
				table.Render()
			}

		})
	},
}
