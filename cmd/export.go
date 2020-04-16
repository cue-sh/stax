package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/pkg/encoding/yaml"
	"github.com/TangoGroup/stx/stx"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Exports cue templates as CloudFormation yml files.",
	Long: `Export will operate over every stack found in the evaluated cue files.
	
The following config.stx.cue options are avilable:

Cmd: {
	Export: YmlPath: string | *"./yml"
}

The YmlPath is a path relative to the cue root. The cue root is the folder that
contains the cue.mod folder. For example, to store files according to the
following tree, set Cmd: Export: YmlPath: "../yml/cloudformation"

infrastructure/
|-cue
| |-cue.mod
| ...
|-yml
	|-cloudformation

`,
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Flush()

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
				_, saveErr := saveStackAsYml(stack, buildInstance, stackValue)
				if saveErr != nil {
					log.Error(saveErr)
				}
			}
		})
	},
}

func saveStackAsYml(stack stx.Stack, buildInstance *build.Instance, stackValue cue.Value) (string, error) {
	dir := filepath.Clean(config.CueRoot + "/" + config.Cmd.Export.YmlPath + "/" + stack.Profile)
	os.MkdirAll(dir, 0755)

	fileName := dir + "/" + stack.Name + ".cfn.yml"
	log.Infof("%s %s %s %s\n", au.White("Exported"), au.Magenta(stack.Name), au.White("⤏"), fileName)
	template := stackValue.Lookup("Template")
	yml, ymlErr := yaml.Marshal(template)
	if ymlErr != nil {
		return "", ymlErr
	}
	writeErr := ioutil.WriteFile(fileName, []byte(yml), 0644)
	if writeErr != nil {
		return "", writeErr
	}
	return fileName, nil
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
