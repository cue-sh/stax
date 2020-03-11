package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exports cue templates that implement the Stack pattern as yml files.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Flush()

		buildInstances := stx.GetBuildInstances(args, "cfn")

		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue, flags)
			if stacks != nil {
				for stackName, stack := range stacks {
					_, saveErr := saveStackAsYml(stackName, stack, buildInstance, cueValue)
					if saveErr != nil {
						log.Error(saveErr)
					}
				}
			}
		})
	},
}

func saveStackAsYml(stackName string, stack stx.Stack, buildInstance *build.Instance, cueValue cue.Value) (string, error) {
	dir := filepath.Clean(config.CueRoot + "/" + config.Export.YmlPath + "/" + stack.Profile)
	os.MkdirAll(dir, 0755)

	fileName := dir + "/" + stackName + ".cfn.yml"
	log.Infof("%s %s %s %s\n", au.White("Exported"), au.Magenta(stackName), au.White("⤏"), fileName)
	template := cueValue.Lookup("Stacks", stackName, "Template")
	yml, ymlErr := yaml.Marshal(template)
	if ymlErr != nil {
		return "", ymlErr
	}
	writeErr := ioutil.WriteFile(fileName, yml, 0644)
	if writeErr != nil {
		return "", writeErr
	}
	return fileName, nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
