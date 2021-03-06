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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Returns a stack status for each stack",
	Long: `status operates on every stack found in the evaluated cue file.

For each stack, status will query CloudFormation and return the current status.
If the stack does not exist status will return an error.
`,
	Run: func(cmd *cobra.Command, args []string) {
		//TODO add debug messages
		log.Debug("status command executing...")
		defer log.Flush()

		buildInstances := internal.GetBuildInstances(args, config.PackageName)

		internal.Process(config, buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			log.Debug("status command processing...")
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
				cfn := internal.GetCloudFormationClient(stack.Profile, stack.Region)

				// use a struct to pass a string, it's GC'd!
				log.Debug("Describing", stack.Name)
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}
				describeStacksOutput, describeStacksErr := cfn.DescribeStacks(context.TODO(), &describeStacksInput)
				log.Debugf("describeStacksOutput:\n%+v\n", describeStacksOutput)
				if describeStacksErr != nil {
					log.Error(describeStacksErr)
					continue
				}

				describedStack := describeStacksOutput.Stacks[0]
				status := string(describedStack.StackStatus)

				table := tablewriter.NewWriter(os.Stdout)
				table.SetAutoWrapText(false)
				table.SetHeader([]string{"Stackname", "Status", "Created", "Updated", "Reason"})
				table.SetHeaderColor(tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor}, tablewriter.Colors{tablewriter.FgWhiteColor})

				if strings.Contains(status, "FAIL") || strings.Contains(status, "ROLLBACK") {
					status = au.Red(status).String()
				} else if strings.Contains(status, "COMPLETE") {
					status = au.BrightGreen(status).String()
				}

				lastUpdatedTime := "Never"
				if describedStack.LastUpdatedTime != nil {
					lastUpdatedTime = describedStack.LastUpdatedTime.Local().String()
				}

				table.Append([]string{au.Magenta(stack.Name).String(), status, describedStack.CreationTime.Local().String(), lastUpdatedTime, aws.ToString(describedStack.StackStatusReason)})
				table.Render()
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
