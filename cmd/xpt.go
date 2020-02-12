package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"cuelang.org/go/cue"

	"cuelang.org/go/cue/build"
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
		loadCueInstances(args, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := getStacks(cueValue)
			if stacks != nil {
				//fmt.Printf("%+v\n\n", top)
				au := aurora.NewAurora(true)
				for stackName, stack := range stacks {
					pattern := ".*"
					if environment != "" || regionCode != "" {
						pattern = ""
						if environment != "" {
							pattern = pattern + "^(" + environment + ")"
						}
						if regionCode != "" {
							pattern = pattern + "(" + regionCode + ")$"
						}
					}
					environmentMatch, _ := regexp.MatchString(pattern, stackName)
					if environmentMatch {
						dir := filepath.Clean(buildInstance.Root + "/../yml/cfn/" + stack.Profile)
						os.MkdirAll(dir, 0766)
						//fmt.Println(err)
						fileName := dir + "/" + stackName + ".cfn.yml"
						fmt.Printf("%s %s %s %s\n", au.White("Saving"), au.Magenta(stackName), au.White("⤏"), fileName)
						template := cueValue.Lookup(stackName, "Template")
						yml, _ := yaml.Marshal(template)
						//fmt.Printf("%+v", string(yml))
						ioutil.WriteFile(fileName, yml, 0766)
					}
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(xptCmd)
}
