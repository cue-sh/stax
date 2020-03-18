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

		log.Debug("Print command running...")
		if flags.PrintOnlyErrors && flags.PrintHideErrors {
			log.Fatal("Cannot show only errors while hiding them.")
		}
		log.Debug("Getting build instances...")
		buildInstances := stx.GetBuildInstances(args, "cfn")
		log.Debug("Processing build instances...")
		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {

			log.Debug("Getting stacks...")
			//stacks, stacksErr := stx.GetStacks(cueValue, flags)

			stacksIterator, stacksIteratorErr := stx.NewStacksIterator(cueValue, flags)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}
			log.Info(au.Cyan(buildInstance.DisplayPath))
			for stacksIterator.Next() {
				stack := stacksIterator.Value()
				valueToMarshal := stack
				path := []string{}
				displayPath := ""

				if flags.PrintPath != "" {
					path = append(path, strings.Split(flags.PrintPath, ".")...)
					valueToMarshal = cueValue.Lookup(path...)
					displayPath = strings.Join(path, ".") + ":\n"
					if !valueToMarshal.Exists() {
						continue
					}
				}

				yml, ymlErr := yaml.Marshal(valueToMarshal)
				stackName, _ = stack.Label()
				log.Info(au.Magenta(stackName))
				if ymlErr != nil {
					if !flags.PrintHideErrors {
						log.Error(ymlErr)
					}
				} else {
					if !flags.PrintOnlyErrors {
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
