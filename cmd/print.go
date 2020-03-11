package cmd

import (
	"strings"

	"github.com/TangoGroup/stx/stx"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/pkg/encoding/yaml"
	"github.com/spf13/cobra"
)

// printCmd represents the print command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Prints the Cue output as YAML",
	Long:  `yada yada yada`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		if flags.PrintOnlyErrors && flags.PrintHideErrors {
			log.Fatal("Cannot show only errors while hiding them.")
		}

		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			valueToMarshal := cueValue
			stacks := stx.GetStacks(cueValue, flags)

			for stackName := range stacks {
				var path []string
				var displayPath string
				if flags.PrintPath != "" {
					path = []string{"Stacks", stackName}
					path = append(path, strings.Split(flags.PrintPath, ".")...)
					valueToMarshal = cueValue.Lookup(path...)
					if valueToMarshal.Err() != nil {
						continue
					}
					displayPath = strings.Join(path, ".") + ":\n"
				}
				yml, ymlErr := yaml.Marshal(valueToMarshal)

				if ymlErr != nil {
					if !flags.PrintHideErrors {
						log.Info(au.Cyan(buildInstance.DisplayPath))
						log.Error(ymlErr.Error())
					}
				} else {
					if !flags.PrintOnlyErrors {
						log.Info(au.Cyan(buildInstance.DisplayPath))
						log.Infof("%s\n", displayPath+string(yml))
					}
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(printCmd)

	printCmd.Flags().BoolVar(&flags.PrintOnlyErrors, "only-errors", false, "Only print errors. Cannot be used in concjunction with --hide-errors")
	printCmd.Flags().BoolVar(&flags.PrintHideErrors, "hide-errors", false, "Hide errors. Cannot be used in concjunction with --only-errors")
	printCmd.Flags().StringVarP(&flags.PrintPath, "path", "p", "", "Dot-notation style path to key to print. Eg: Template.Resources.Alb or Template.Outputs")

}
