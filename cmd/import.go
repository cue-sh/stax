package cmd

import (
	"cuelang.org/go/encoding/json"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVar(&flags.ImportStack, "stack", "", "Stack name to import. (Required)")
	importCmd.MarkFlagRequired("stack")
	importCmd.Flags().StringVar(&flags.ImportRegion, "region", "", "Region where stack is located. (Required)")
	importCmd.MarkFlagRequired("region")
}

// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Imports an existing stack into Cue.",
	Long:  `yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		if flags.Profile == "" {
			log.Error("--profile is required for import")
		}

		flags.StackNameRegexPattern = "^" + flags.ImportStack + "$"

		stx.EnsureVaultSession(config)

		// get a session and cloudformation service client
		session := stx.GetSession(flags.Profile)
		cfn := cloudformation.New(session, aws.NewConfig().WithRegion(flags.ImportRegion))
		log.Infof("%s %s...", au.White("Importing"), au.Magenta(flags.ImportStack))

		getTemplateOutput, getTemplateErr := cfn.GetTemplate(&cloudformation.GetTemplateInput{StackName: aws.String(flags.ImportStack), TemplateStage: aws.String("Processed")})

		if getTemplateErr != nil {
			log.Error(getTemplateErr)
			return
		}

		templateBytes := []byte(aws.StringValue(getTemplateOutput.TemplateBody))

		if !json.Valid(templateBytes) {
			// it must be yaml so convert to json
			var yamlErr error
			templateBytes, yamlErr = yaml.YAMLToJSON(templateBytes)
			if yamlErr != nil {
				log.Error(yamlErr)
				return
			}
		}

		templateErr := createTemplate(args, string(templateBytes))
		if templateErr != nil {
			log.Error(templateErr)
		}
		log.Check()
	},
}

// all-eks-deployer-kubectl-layer-usw2
