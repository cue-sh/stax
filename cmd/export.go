package cmd

import (
	"fmt"
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
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue, flags)
			if stacks != nil {
				for stackName, stack := range stacks {
					saveStackAsYml(stackName, stack, buildInstance, cueValue)
				}
			}
		})
	},
}

func saveStackAsYml(stackName string, stack stx.Stack, buildInstance *build.Instance, cueValue cue.Value) string {
	dir := filepath.Clean(config.CueRoot + "/" + config.Export.YmlPath + "/" + stack.Profile)
	os.MkdirAll(dir, 0755)
	//fmt.Println(err)
	fileName := dir + "/" + stackName + ".cfn.yml"
	fmt.Printf("%s %s %s %s\n", au.White("Exported"), au.Magenta(stackName), au.White("⤏"), fileName)
	template := cueValue.Lookup("Stacks", stackName, "Template")
	yml, _ := yaml.Marshal(template)
	//fmt.Printf("YAML: %+v\n", string(yml))
	ioutil.WriteFile(fileName, yml, 0644)
	return fileName
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
