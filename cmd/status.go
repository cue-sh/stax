package cmd

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/spf13/cobra"
)

var stackName string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Returns a stack status if it exists",
	Long:  `How long...?`,
	Run: func(cmd *cobra.Command, args []string) {

		//if stackName == "" {
		//	fmt.Println(au.Red("Please provide a stack name."))
		//	os.Exit(1)
		//}
		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			stacks := stx.GetStacks(cueValue, flags)

			for stackName, stack := range stacks {
				fmt.Println(stackName)
				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

				// use a struct to pass a string, it's GC'd!
				describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
				describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)
				if describeStacksErr != nil {
					fmt.Println(describeStacksErr)
					fmt.Println(describeStacksOutput)
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
