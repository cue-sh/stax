package cmd

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/encoding/json"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(importCmd)
}

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Imports an existing stack into Cue.",
	Long:  `yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {

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
				log.Infof("%s %s...", au.White("Importing"), au.Magenta(stack.Name))

				getTemplateOutput, getTemplateErr := cfn.GetTemplate(&cloudformation.GetTemplateInput{StackName: aws.String(stack.Name), TemplateStage: aws.String("Processed")})

				if getTemplateErr != nil {
					log.Error(getTemplateErr)
					continue
				}
				log.Check()
				log.Infof("%s\n", aws.StringValue(getTemplateOutput.TemplateBody))
				jsonBytes := []byte(aws.StringValue(getTemplateOutput.TemplateBody))
				expression, extractErr := json.Extract("", jsonBytes)
				if extractErr != nil {
					log.Error(extractErr)
					continue
				}
				log.Infof("%+v\n", expression)

				newCueInstance, fillErr := cueInstance.Fill(expression, "Stacks", stack.Name, "Template")

				if fillErr != nil {
					log.Error(fillErr)
					continue
				}

				result, formatErr := format.Node(newCueInstance.Value().Syntax(), format.Simplify())
				if formatErr != nil {
					log.Error(formatErr)
					continue
				}
				log.Infof("%s\n", result)
			}
		})
	},
}

// all-eks-deployer-kubectl-layer-usw2
