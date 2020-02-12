package cmd

import (
	"fmt"

	"github.com/TangoGroup/stx/stx"
	"github.com/logrusorgru/aurora"

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
		au := aurora.NewAurora(true)
		stx.LoadCueInstances(args, "cfn", func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			fmt.Println(au.Cyan(buildInstance.DisplayPath))
			yml, _ := yaml.Marshal(cueValue)
			fmt.Printf("%s\n", string(yml))
		})
	},
}

func init() {
	rootCmd.AddCommand(printCmd)
}
