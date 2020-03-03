package cmd

import (
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var stackName string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Returns a stack status if it exists",
	Long:  `How long...?`,
	Run: func(cmd *cobra.Command, args []string) {

		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			stacks := stx.GetStacks(cueValue, flags)

			for stackName, stack := range stacks {
				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

				// use a struct to pass a string, it's GC'd!
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
				describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)
				if describeStacksErr != nil {
					fmt.Println(describeStacksErr)
				}
				status := describeStacksOutput.Stacks[0].StackStatus
				table := tablewriter.NewWriter(os.Stdout)
				table.SetAutoWrapText(false)
				table.SetHeader([]string{"Stackname", "Status"})
				table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})
				if *status == "CREATE_COMPLETE" {
					table.Append([]string{stackName, *status + "🤘"})
				} else {
					table.Append([]string{stackName, *status})
				}

				table.Render()
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
