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

// eventsCmd represents the events command
var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Shows the latest events from the evaluated stacks.",
	Long:  `Yaba daba doo.`,
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
				describeStackEventsInput := cloudformation.DescribeStackEventsInput{StackName: aws.String(stack.Name)}
				describeStackEventsOutput, describeStackEventsErr := cfn.DescribeStackEvents(&describeStackEventsInput)
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
						reason = aws.StringValue(event.ResourceStatusReason)
					}
					status := aws.StringValue(event.ResourceStatus)
					if strings.Contains(aws.StringValue(event.ResourceStatus), "COMPLETE") {
						status = au.BrightGreen(aws.StringValue(event.ResourceStatus)).String()
					}
					// if strings.Contains(aws.StringValue(event.ResourceStatus), "PROGRESS") {
					// 	status = au.Yellow(aws.StringValue(event.ResourceStatus)).String()
					// }
					if strings.Contains(aws.StringValue(event.ResourceStatus), "FAIL") || strings.Contains(aws.StringValue(event.ResourceStatus), "ROLLBACK") {
						status = au.Red(aws.StringValue(event.ResourceStatus)).String()
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
