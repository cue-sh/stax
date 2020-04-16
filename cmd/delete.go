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

func init() {
	rootCmd.AddCommand(deleteCmd)
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes the stack along with .yml and .out.cue files",
	Long: `delete will operate on every stack found among the evaluated cue files.

For each stack, delete will—as the name suggests—DELETE the stack!

Beware that the only safety mechanism provided is a requirement to enter the
stack name (case-sensitive match).

** It your responsibility to ensure the proper authorization policies are 
applied to the credentials being used! **
`,
	Run: func(cmd *cobra.Command, args []string) {

		//TODO add debug messages
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

				log.Infof("%s %s?\nEnter the name of the stack to confirm.\n▶︎", au.Red("DELETE"), au.Magenta(stack.Name))
				var input string
				fmt.Scanln(&input)

				if input != stack.Name {
					continue
				}

				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

				// use a struct to pass a string, it's GC'd!
				deleteStackInput := cloudformation.DeleteStackInput{StackName: aws.String(stack.Name)}
				_, deleteStackErr := cfn.DeleteStack(&deleteStackInput)
				if deleteStackErr != nil {
					log.Error(deleteStackErr)
					continue
				}

			}
		})
	},
}
