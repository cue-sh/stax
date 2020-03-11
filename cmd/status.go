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

var stackName string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Returns a stack status if it exists",
	Long:  `How long...?`,
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Flush()
		//TODO add debug messages
		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			stacks, stacksErr := stx.GetStacks(cueValue, flags)
			if stacksErr != nil {
				log.Error(stacksErr)
			}

			if stacks == nil {
				return
			}

			for stackName, stack := range stacks {
				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

				// use a struct to pass a string, it's GC'd!
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
				describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)
				if describeStacksErr != nil {
					log.Error(describeStacksErr)
					continue
				}

				describedStack := describeStacksOutput.Stacks[0]
				status := aws.StringValue(describedStack.StackStatus)

				table := tablewriter.NewWriter(os.Stdout)
				table.SetAutoWrapText(false)
				table.SetHeader([]string{"Stackname", "Status", "Created", "Updated", "Reason"})
				table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})

				if strings.Contains(status, "COMPLETE") {
					status = au.BrightGreen(status).String()
				}

				if strings.Contains(status, "FAIL") || strings.Contains(status, "ROLLBACK") {
					status = au.Red(status).String()
				}
				table.Append([]string{au.Magenta(stackName).String(), status, describedStack.CreationTime.Local().String(), describedStack.LastUpdatedTime.Local().String(), aws.StringValue(describedStack.StackStatusReason)})
				table.Render()
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
