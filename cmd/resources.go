package cmd

import (
	"context"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
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
				// get a session and cloudformation service client
				cfn := internal.GetCloudFormationClient(stack.Profile, stack.Region)
				log.Infof("%s %s...\n", au.White("Describing"), au.Magenta(stack.Name))

				describeStackResourcesInput := cloudformation.DescribeStackResourcesInput{StackName: aws.String(stack.Name)}
				describeStackResourcesOutput, describeStackResourcesErr := cfn.DescribeStackResources(context.TODO(), &describeStackResourcesInput)
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

					status := string(resource.ResourceStatus)
					if strings.Contains(string(resource.ResourceStatus), "COMPLETE") {
						status = au.BrightGreen(string(resource.ResourceStatus)).String()
					}

					if strings.Contains(string(resource.ResourceStatus), "FAIL") || strings.Contains(string(resource.ResourceStatus), "ROLLBACK") {
						status = au.Red(string(resource.ResourceStatus)).String()
					}

					table.Append([]string{aws.ToString(resource.LogicalResourceId), aws.ToString(resource.PhysicalResourceId), aws.ToString(resource.ResourceType), status})
				}
				table.Render()
			}

		})
	},
}
