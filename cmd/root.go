package cmd

import (
	"fmt"
	"os"

	"github.com/cue-sh/stax/internal"
	"github.com/cue-sh/stax/logger"
	"github.com/logrusorgru/aurora"

	"github.com/spf13/cobra"
)

var au aurora.Aurora        // console output color
var config *internal.Config // holds settings in config.stax.cue files
var flags internal.Flags    // holds command line flags
var log *logger.Logger      // commong log

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "stax",
	Short: "Export and deploy CUE-based CloudFormation stacks.",
	Long:  `Yada yada yada.`,
	// Run: func(cmd *cobra.Command, args []string) {},
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
	cobra.OnInitialize(func() {
		au = aurora.NewAurora(!flags.NoColor)
		log = logger.NewLogger(flags.Debug, flags.NoColor)

		if config == nil {
			log.Debug("Loading config...")
			config = internal.LoadConfig(log)
		}
		log.Debugf("Loaded flags %+v\n", flags)
		log.Debug("Root command initialized.")
	})

	flags = internal.Flags{}
	rootCmd.PersistentFlags().StringVarP(&flags.Environment, "environment", "e", "", "Includes only stacks with this environment.")
	rootCmd.PersistentFlags().StringVar(&flags.Profile, "profile", "", "Includes only stacks with this profile")
	rootCmd.PersistentFlags().StringVarP(&flags.RegionCode, "region-code", "r", "", "Includes only stacks with this region code")
	rootCmd.PersistentFlags().StringVar(&flags.Exclude, "exclude", "", "Excludes subdirectory paths matching this regular expression.")
	rootCmd.PersistentFlags().StringVar(&flags.Include, "include", "", "Includes subdirectory paths matching this regular expression.")
	rootCmd.PersistentFlags().StringVar(&flags.StackNameRegexPattern, "stacks", "", "Includes only stacks whose name matches this regular expression.")
	rootCmd.PersistentFlags().StringVar(&flags.Has, "has", "", "Includes only stacks that contain the provided path. E.g.: Template.Parameters")
	rootCmd.PersistentFlags().BoolVar(&flags.Debug, "debug", false, "Enables verbose output of debug level messages.")
	rootCmd.PersistentFlags().BoolVar(&flags.NoColor, "no-color", false, "Disables color output.")
}
