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
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

// xptCmd represents the xpt command
var xptCmd = &cobra.Command{
	Use:   "xpt",
	Short: "eXPorTs cue templates that implement the Stacks:[] pattern.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {
		au := aurora.NewAurora(true)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue)
			if stacks != nil {
				for stackName, stack := range stacks {
					dir := filepath.Clean(buildInstance.Root + "/../yml/cfn/" + stack.Profile)
					os.MkdirAll(dir, 0766)
					//fmt.Println(err)
					fileName := dir + "/" + stackName + ".cfn.yml"
					fmt.Printf("%s %s %s %s\n", au.White("Saving"), au.Magenta(stackName), au.White("⤏"), fileName)
					template := cueValue.Lookup("Stacks", stackName, "Template")
					yml, _ := yaml.Marshal(template)
					//fmt.Printf("YAML: %+v\n", string(yml))
					ioutil.WriteFile(fileName, yml, 0766)
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(xptCmd)
}
