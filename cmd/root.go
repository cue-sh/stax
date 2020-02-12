package cmd

import (
	"fmt"
	"os"

	"github.com/logrusorgru/aurora"

	"github.com/spf13/cobra"
)

var environment, regionCode string
var au aurora.Aurora

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
	au = aurora.NewAurora(true)
	rootCmd.PersistentFlags().StringVarP(&environment, "environment", "e", "", "Any environment listed in maps/Environments.cue")
	rootCmd.PersistentFlags().StringVarP(&regionCode, "region-code", "r", "", "Any region code listed in maps/RegionCodes.cue")

	//loadConfig()

}

// TODO: figure out how to do a single config file
// const configCue = `
// {
// 	awsVault: bool
// 	xpt: ymlPath: string | *"../yml"
// }
// `

// var configValue cue.Value

// func loadConfig() {

// 	wd, _ := os.Getwd()
// 	separator := string(os.PathSeparator)
// 	dirs := strings.Split(wd, separator)
// 	dirsLen := len(dirs)
// 	var path string
// 	// traverse the directory tree starting from PWD going up to successive parents
// 	for i := dirsLen; i > 0; i-- {
// 		path = strings.Join(dirs[:i], separator)
// 		// look for the cue.mod filder
// 		if _, err := os.Stat(path + "/cue.mod"); !os.IsNotExist(err) {
// 			break // found it!
// 		}
// 	}

// 	var buildInstances []*build.Instance
// 	var cueInstances []*cue.Instance
// 	var configInstance, userConfigInstance *cue.Instance

// 	// include baked-in cue config
// 	var runtime cue.Runtime
// 	configInstance, _ = runtime.Compile("stxConfig", configCue)
// 	configValue = configInstance.Value()
// 	// expect config.stx.cue to be colocated with cue.mod
// 	configPath := path + "/config.stx.cue"
// 	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
// 		buildInstances = stx.GetBuildInstances([]string{configPath}, "stx")
// 		cueInstances = cue.Build(buildInstances)
// 		userConfigInstance = cueInstances[0]
// 		configValue = configValue.Unify(userConfigInstance.Value())
// 	}

// 	configErr := configValue.Err()
// 	if configErr != nil {
// 		au := aurora.NewAurora(true)
// 		fmt.Println(au.Red("Config error: " + configErr.Error()))
// 		os.Exit(1)
// 	}

// 	fmt.Printf("%+v", configValue)
// 	os.Exit(1)
// }
