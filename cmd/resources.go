package cmd

import (
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/cue-sh/stax/internal"
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
	Long: `Resources operates on every stack found in the evaluated cue files.
	
For each stack, resources will query CloudFormation and return a list of all
resources currently managed in the stack.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// TODO add debug messages
		defer log.Flush()
		internal.EnsureVaultSession(config)

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

				// get a session and cloudformation service client
				session := internal.GetSession(stack.Profile)
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
