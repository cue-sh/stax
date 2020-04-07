package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

const scaffold = `package cfn

Stacks: {
	[StackName= =~"${STX::StackNameRegexPattern}"]: {
		stack=Stacks[StackName]
		Profile: "${STX::Profile}"
		Environment: "${STX::Environment}"
		RegionCode: "${STX::RegionCode}"
		Template: {
			Outputs: {}
			Resources: {}
		}
	}	
}`

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [path/to/template.cfn.cue]",
	Short: "Writes scaffolding to template.cfn.cue",
	Long: `Path to template will default to ./template.cfn.cue
	
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
		var pathToTemplate string

		output := strings.Replace(scaffold, "${STX::StackNameRegexPattern}", flags.StackNameRegexPattern, 1)
		output = strings.Replace(output, "${STX::Profile}", flags.Profile, 1)
		output = strings.Replace(output, "${STX::Environment}", flags.Environment, 1)
		output = strings.Replace(output, "${STX::RegionCode}", flags.RegionCode, 1)

		if len(args) < 1 {
			pathToTemplate = "./template.cfn.cue"
		} else {
			pathToTemplate = args[0]
		}

		path := filepath.Dir(pathToTemplate)
		os.MkdirAll(path, 0766)

		writeErr := ioutil.WriteFile(pathToTemplate, []byte(output), 0655)
		if writeErr != nil {
			log.Fatal(writeErr)
		}
	},
}
