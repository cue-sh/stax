package cmd

import (
	"fmt"
	"os"

	"github.com/TangoGroup/stx/stx"
	"github.com/logrusorgru/aurora"

	"github.com/spf13/cobra"
)

var au aurora.Aurora // TODO move au to Logger pacakge
// config and flgs should be the only global vars
var config stx.Config // holds settings in config.stx.cue files
var flags stx.Flags   // holds command line flags

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

	flags = stx.Flags{}
	rootCmd.PersistentFlags().StringVarP(&flags.Environment, "environment", "e", "", "Includes only stacks with this environment.")
	rootCmd.PersistentFlags().StringVar(&flags.Profile, "profile", "", "Includes only stacks with this profile")
	rootCmd.PersistentFlags().StringVarP(&flags.RegionCode, "region-code", "r", "", "Includes only stacks with this region code")
	rootCmd.PersistentFlags().StringVarP(&flags.Exclude, "exclude", "x", "", "Excludes subdirectory paths matching this regular expression.")
	rootCmd.PersistentFlags().StringVarP(&flags.Include, "include", "n", "", "Includes subdirectory paths matching this regular expression.")

	au = aurora.NewAurora(true)
	config = stx.LoadConfig()
}
