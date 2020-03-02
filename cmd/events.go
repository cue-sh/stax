package cmd

import (
	"fmt"
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
		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue, flags)
			if stacks != nil {
				for stackName, stack := range stacks {
					// get a session and cloudformation service client
					session := stx.GetSession(stack.Profile)
					cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))
					sn := stackName // to avoid issues with pointing to a for-scoped var
					describeStackEventsInput := cloudformation.DescribeStackEventsInput{StackName: &sn}
					describeStackEventsOutput, describeStackEventsErr := cfn.DescribeStackEvents(&describeStackEventsInput)
					if describeStackEventsErr != nil {
						fmt.Println(au.Red(describeStackEventsErr))
						continue
					}
					// fmt.Printf("%+v\n", describeStackEventsOutput)

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
							reason = *event.ResourceStatusReason
						}
						status := *event.ResourceStatus
						if strings.Contains(*event.ResourceStatus, "COMPLETE") {
							status = au.BrightGreen(*event.ResourceStatus).String()
						}
						// if strings.Contains(*event.ResourceStatus, "PROGRESS") {
						// 	status = au.Yellow(*event.ResourceStatus).String()
						// }
						if strings.Contains(*event.ResourceStatus, "FAIL") || strings.Contains(*event.ResourceStatus, "ROLLBACK") {
							status = au.Red(*event.ResourceStatus).String()
							reason = au.Red(reason).String()
						}
						resource := *event.LogicalResourceId
						if strings.Contains(*event.LogicalResourceId, stackName) {
							resource = au.Magenta(resource).String()
						}

						table.Append([]string{resource, status, event.Timestamp.Local().String(), reason})
					}

					table.Render()
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)

	eventsCmd.Flags().IntP("number", "n", 5, "The number of events to display. Setting this < 0 will display all events")
}
