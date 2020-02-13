package stx

import (
	"fmt"
	"os"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/logrusorgru/aurora"
)

const configCue = `
{
	Xpt: YmlPath: string | *"./yml"
}
`

// Config holds config values parsed from config.stx.cue files
type Config struct {
	CueRoot string
	Xpt     struct {
		YmlPath string
	}
}

// LoadConfig looks for config.stx.cue to be colocated with cue.mod and unifies that with a built-in default config schema
func LoadConfig() Config {

	wd, _ := os.Getwd()
	separator := string(os.PathSeparator)
	dirs := strings.Split(wd, separator)
	dirsLen := len(dirs)
	var path string
	// traverse the directory tree starting from PWD going up to successive parents
	for i := dirsLen; i > 0; i-- {
		path = strings.Join(dirs[:i], separator)
		// look for the cue.mod filder
		if _, err := os.Stat(path + "/cue.mod"); !os.IsNotExist(err) {
			break // found it!
		}
	}

	var buildInstances []*build.Instance
	var cueInstances []*cue.Instance
	var configInstance, userConfigInstance *cue.Instance
	var configValue cue.Value

	// include baked-in cue config
	var runtime cue.Runtime
	configInstance, _ = runtime.Compile("stxConfig", configCue)
	configValue = configInstance.Value()
	// expect config.stx.cue to be colocated with cue.mod
	configPath := path + "/config.stx.cue"
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		buildInstances = GetBuildInstances([]string{configPath}, "stx")
		cueInstances = cue.Build(buildInstances)
		userConfigInstance = cueInstances[0]
		configValue = configValue.Unify(userConfigInstance.Value())
	}

	configErr := configValue.Err()
	if configErr != nil {
		au := aurora.NewAurora(true)
		fmt.Println(au.Red("Config error: " + configErr.Error()))
		os.Exit(1)
	}

	cfg := Config{CueRoot: path}

	decodeErr := configValue.Decode(&cfg)
	if decodeErr != nil {
		fmt.Println(decodeErr.Error())
	}

	return cfg
}
