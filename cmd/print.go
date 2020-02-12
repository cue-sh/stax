package cmd

import (
	"fmt"

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
		loadCueInstances(args, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			fmt.Println(buildInstance.DisplayPath)
			yml, _ := yaml.Marshal(cueValue)
			fmt.Printf("%s\n", string(yml))
		})
	},
}

func init() {
	rootCmd.AddCommand(printCmd)
}
