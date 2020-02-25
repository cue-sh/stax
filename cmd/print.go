package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/TangoGroup/stx/stx"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/pkg/encoding/yaml"
	"github.com/spf13/cobra"
)

var printOnlyErrors, printHideErrors bool
var printPath string

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints the Cue output as YAML",
	Long:  `yada yada yada`,
	Run: func(cmd *cobra.Command, args []string) {

		if printOnlyErrors && printHideErrors {
			fmt.Println(au.Red("Cannot show only errors while hiding them."))
			os.Exit(1)
		}
		totalErrors := 0
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags.exclude, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			valueToMarshal := cueValue
			stacks := stx.GetStacks(cueValue)

			for stackName := range stacks {
				var path []string
				var displayPath string
				if printPath != "" {
					path = []string{"Stacks", stackName}
					path = append(path, strings.Split(printPath, ":")...)
					valueToMarshal = cueValue.Lookup(path...)
					if valueToMarshal.Err() != nil {
						continue
					}
					displayPath = strings.Join(path, ":") + ":\n"
				}
				yml, ymlErr := yaml.Marshal(valueToMarshal)

				if ymlErr != nil {
					totalErrors++
					if !printHideErrors {
						fmt.Println(au.Cyan(buildInstance.DisplayPath))
						fmt.Println(au.Red(ymlErr.Error()))
					}
				} else {
					if !printOnlyErrors {
						fmt.Println(au.Cyan(buildInstance.DisplayPath))
						fmt.Printf("%s\n", displayPath+string(yml))
					}
				}
			}
		})

		if !printHideErrors && totalErrors > 0 {
			fmt.Println("Total errors: ", totalErrors)
		}
	},
}

func init() {
	rootCmd.AddCommand(printCmd)
	// TODO add flag to skip/hide errors

	printCmd.Flags().BoolVar(&printOnlyErrors, "only-errors", false, "Only print errors. Cannot be used in concjunction with --hide-errors")
	printCmd.Flags().BoolVar(&printHideErrors, "hide-errors", false, "Hide errors. Cannot be used in concjunction with --only-errors")
	printCmd.Flags().StringVarP(&printPath, "path", "p", "", "Dot-notation style path to key to print. Eg: Template.Resources.Alb")

}
