package internal

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/cue-sh/stax/logger"
)

// Flags holds flags passed in from cli
type Flags struct {
	Environment, Profile, RegionCode, Exclude, Include, StackNameRegexPattern, Has, PrintPath, ImportStack, ImportRegion string
	Debug, NoColor                                                                                                       bool
	PrintOnlyErrors, PrintHideErrors, PrintOnlyNames, PrintHidePath, PrintOnlyPaths                                      bool
	DeployWait, DeploySave, DeployDeps, DeployPrevious, DeployNoExecute, DeployExecuteOnly                               bool
}

const configCue = `package stax
PackageName: string | *"cfn"
Cmd: {
	Export: YmlPath: string | *"./yml"
	Save: {
		OutFilePrefix: string | *""
	}
}
`

// Config holds config values parsed from config.stax.cue files
type Config struct {
	CueRoot     string
	OsSeparator string
	PackageName string
	Cmd         struct {
		Export struct {
			YmlPath string
		}
		Save struct {
			OutFilePrefix string
		}
	}
}

// LoadConfig looks for config.stax.cue to be colocated with cue.mod and unifies that with a built-in default config schema
func LoadConfig(log *logger.Logger) *Config {
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
	configSchema := "/tmp/config.stax.cue"
	ioutil.WriteFile(configSchema, []byte(configCue), 0766)
	buildArgs = append(buildArgs, configSchema)

	// look for global config in ~/.stax/config.stax.cue
	homeConfigPath := filepath.Clean(usr.HomeDir + "/.stax/config.stax.cue")
	if _, err := os.Stat(homeConfigPath); !os.IsNotExist(err) {
		log.Debug("Global config found:", homeConfigPath)
		buildArgs = append(buildArgs, homeConfigPath)
	} else {
		log.Debug("Global config NOT found:", homeConfigPath)
	}

	// look for config.stax.cue colocated with cue.mod
	localConfigPath := path + "/config.stax.cue"
	if _, err := os.Stat(localConfigPath); !os.IsNotExist(err) {
		log.Debug("Local config found:", localConfigPath)
		buildArgs = append(buildArgs, localConfigPath)
	} else {
		log.Debug("Local config NOT found:", localConfigPath)
	}

	log.Debug("Building config...")
	buildInstances = GetBuildInstances(buildArgs, "stax")
	cueInstances = cue.Build(buildInstances)
	configInstance = cueInstances[0]
	configValue = configInstance.Value()

	configErr := configValue.Err()
	if configErr != nil {
		log.Fatal(configErr.Error())
	}

	cfg := Config{CueRoot: path, OsSeparator: separator}

	log.Debug("Decoding config...")
	decodeErr := configValue.Decode(&cfg)
	if decodeErr != nil {
		log.Fatal("Config decode error", decodeErr.Error())
	}
	log.Debugf("Loaded config %+v\n", cfg)
	return &cfg
}
