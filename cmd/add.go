package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/format"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

const scaffold = `package cfn

Stacks: {
	${STX::ImportedStack}
	[StackName= =~"${STX::StackNameRegexPattern}"]: {
		stack=Stacks[StackName]
		Profile: "${STX::Profile}"
		Environment: "${STX::Environment}"
		RegionCode: "${STX::RegionCode}"
		Template: ${STX::Template}
	}	
}`

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [path/to/template.cfn.cue]",
	Short: "Writes scaffolding to the provided path.",
	Long: `add operates on a single stack provided as the path argument.
	
Path to template will default to ./template.cfn.cue
	
The following global flags will be used as so:
--stacks will be used as a regular expression in the template: Stacks: [=~"<stacks>"]
--profile will be used as Stacks: Profile: <profile>
--environment will be used as Stacks: Environment: <environment>
--region-code will be used as Stacks: RegionCode: <regionCode>

The following global flags are ignored: 
--include
--exclude`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Flush()
		scaffoldTemplateDefault := `{
			Outputs: {}
			Resources: {}
}`
		templateErr := createTemplate(args, scaffoldTemplateDefault)
		if templateErr != nil {
			log.Error(templateErr)
		}
	},
}

func createTemplate(args []string, template string) error {
	var pathToTemplate string

	output := strings.Replace(scaffold, "${STX::StackNameRegexPattern}", flags.StackNameRegexPattern, 1)
	output = strings.Replace(output, "${STX::Profile}", flags.Profile, 1)
	output = strings.Replace(output, "${STX::Environment}", flags.Environment, 1)
	output = strings.Replace(output, "${STX::RegionCode}", flags.RegionCode, 1)
	output = strings.Replace(output, "${STX::ImportedStack}", "\""+flags.ImportStack+"\": {}", 1)

	output = strings.Replace(output, "${STX::Template}", template, 1)

	if len(args) < 1 {
		pathToTemplate = "./template.cfn.cue"
	} else {
		pathToTemplate = args[0]
	}

	path := filepath.Dir(pathToTemplate)
	os.MkdirAll(path, 0766)

	cueOutput, cueOutputErr := format.Source([]byte(output), format.Simplify())
	if cueOutputErr != nil {
		return cueOutputErr
	}

	writeErr := ioutil.WriteFile(pathToTemplate, cueOutput, 0644)
	if writeErr != nil {
		return writeErr
	}

	return nil
}
