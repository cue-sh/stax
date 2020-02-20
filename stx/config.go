package stx

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/logrusorgru/aurora"
)

const configCue = `package stx
Auth: Ykman: Profile: string | *""
Xpt: YmlPath: string | *"./yml"
Sops: Profile: string | *""
`

// Config holds config values parsed from config.stx.cue files
type Config struct {
	CueRoot     string
	OsSeparator string
	Auth        struct {
		Ykman struct {
			Profile string
		}
	}
	Sops struct {
		Profile string
	}
	Xpt struct {
		YmlPath string
	}
}

// LoadConfig looks for config.stx.cue to be colocated with cue.mod and unifies that with a built-in default config schema
func LoadConfig() Config {

	wd, _ := os.Getwd()
	separator := string(os.PathSeparator)
	dirs := strings.Split(wd, separator)
	dirsLen := len(dirs)
	usr, _ := user.Current()
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
	var configInstance *cue.Instance
	var configValue cue.Value
	var buildArgs []string

	// include baked-in cue config
	configSchema := "/tmp/config.stx.cue"
	ioutil.WriteFile(configSchema, []byte(configCue), 0766)
	buildArgs = append(buildArgs, configSchema)

	// look for global config in ~/.stx/config.stx.cue
	homeConfigPath := filepath.Clean(usr.HomeDir + "/.stx/config.stx.cue")
	if _, err := os.Stat(homeConfigPath); !os.IsNotExist(err) {
		buildArgs = append(buildArgs, homeConfigPath)
	}

	// look for config.stx.cue colocated with cue.mod
	localConfigPath := path + "/config.stx.cue"
	if _, err := os.Stat(localConfigPath); !os.IsNotExist(err) {
		buildArgs = append(buildArgs, localConfigPath)
	}

	buildInstances = GetBuildInstances(buildArgs, "stx")
	cueInstances = cue.Build(buildInstances)
	configInstance = cueInstances[0]
	configValue = configInstance.Value()

	configErr := configValue.Err()
	if configErr != nil {
		au := aurora.NewAurora(true)
		fmt.Println(au.Red("Config error: " + configErr.Error()))
		os.Exit(1)
	}

	cfg := Config{CueRoot: path, OsSeparator: separator}

	decodeErr := configValue.Decode(&cfg)
	if decodeErr != nil {
		fmt.Println(decodeErr.Error())
		os.Exit(1)
	}

	return cfg
}
