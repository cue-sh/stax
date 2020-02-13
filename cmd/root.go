package cmd

import (
	"fmt"
	"os"

	"github.com/TangoGroup/stx/stx"
	"github.com/logrusorgru/aurora"

	"github.com/spf13/cobra"
)

var environment, regionCode string
var au aurora.Aurora
var config stx.Config

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

	rootCmd.PersistentFlags().StringVarP(&environment, "environment", "e", "", "Any environment listed in maps/Environments.cue")
	rootCmd.PersistentFlags().StringVarP(&regionCode, "region-code", "r", "", "Any region code listed in maps/RegionCodes.cue")

	au = aurora.NewAurora(true)
	config = stx.LoadConfig()

}
