package cmd

import (
	"fmt"
	"os"

	"github.com/TangoGroup/stx/stx"
	"github.com/logrusorgru/aurora"

	"github.com/spf13/cobra"
)

var au aurora.Aurora  // TODO move au to Logger pacakge
var config stx.Config // TODO refactor use of global vars

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stx",
	Short: "Export and deploy CUE-based CloudFormation stacks.",
	Long:  `Yada yada yada.`,
	// Run:   func(cmd *cobra.Command, args []string) {},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize()

	rootCmd.PersistentFlags().StringP("environment", "e", "", "Any environment listed in maps/Environments.cue")
	rootCmd.PersistentFlags().StringP("region-code", "r", "", "Any region code listed in maps/RegionCodes.cue")
	rootCmd.PersistentFlags().StringP("exclude", "x", "", "Subdirectory paths matching this regular expression will be ignored.")

	au = aurora.NewAurora(true)
	config = stx.LoadConfig()

}
