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

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Shows the latest events from the evaluated stacks.",
	Long: `Events operates on every stack found in the evaluated cue files.
	
For each stack, events will query CloudFormation and return a list of events.`,
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
				describeStackEventsInput := cloudformation.DescribeStackEventsInput{StackName: aws.String(stack.Name)}
				describeStackEventsOutput, describeStackEventsErr := cfn.DescribeStackEvents(context.TODO(), &describeStackEventsInput)
				if describeStackEventsErr != nil {
					log.Error(describeStackEventsErr)
					continue
				}
				// TODO add --aws-output(?) to be used in conjunction with --debug
				// log.Debugf("%+v\n", describeStackEventsOutput)

				numberStacksToDisplay, _ := cmd.Flags().GetInt("number")
				if numberStacksToDisplay < 0 {
					numberStacksToDisplay = len(describeStackEventsOutput.StackEvents)
				}

				table := tablewriter.NewWriter(os.Stdout)
				table.SetAutoWrapText(false)
				table.SetHeader([]string{"Resource", "Status", "Time", "Reason"})
				table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})

				for i, event := range describeStackEventsOutput.StackEvents {
					if i >= numberStacksToDisplay {
						break
					}
					reason := "-"
					if event.ResourceStatusReason != nil {
						reason = aws.ToString(event.ResourceStatusReason)
					}
					status := string(event.ResourceStatus)
					if strings.Contains(string(event.ResourceStatus), "COMPLETE") {
						status = au.BrightGreen(string(event.ResourceStatus)).String()
					}
					// if strings.Contains(aws.ToString(event.ResourceStatus), "PROGRESS") {
					// 	status = au.Yellow(aws.ToString(event.ResourceStatus)).String()
					// }
					if strings.Contains(string(event.ResourceStatus), "FAIL") || strings.Contains(string(event.ResourceStatus), "ROLLBACK") {
						status = au.Red(string(event.ResourceStatus)).String()
						reason = au.Red(reason).String()
					}
					resource := *event.LogicalResourceId
					if strings.Contains(*event.LogicalResourceId, stack.Name) {
						resource = au.Magenta(resource).String()
					}

					table.Append([]string{resource, status, event.Timestamp.Local().String(), reason})
				}

				table.Render()
			}

		})
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)

	eventsCmd.Flags().IntP("number", "n", 5, "The number of events to display. Setting this < 0 will display all events")
}
