package stx

import (
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/logger"
)

// Flags holds flags passed in from cli
type Flags struct {
	Environment, Profile, RegionCode, Exclude, Include, PrintPath string
	Debug, NoColor, PrintOnlyErrors, PrintHideErrors              bool
}

const configCue = `package stx
Auth: {
	AwsVault: SourceProfile: string | *""
	Ykman: Profile: string | *""
}
Export: YmlPath: string | *"./yml"
`

// Config holds config values parsed from config.stx.cue files
type Config struct {
	CueRoot     string
	OsSeparator string
	Auth        struct {
		AwsVault struct {
			SourceProfile string
		}
		Ykman struct {
			Profile string
		}
	}
	Export struct {
		YmlPath string
	}
}

// LoadConfig looks for config.stx.cue to be colocated with cue.mod and unifies that with a built-in default config schema
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
	configSchema := "/tmp/config.stx.cue"
	ioutil.WriteFile(configSchema, []byte(configCue), 0766)
	buildArgs = append(buildArgs, configSchema)

	// look for global config in ~/.stx/config.stx.cue
	homeConfigPath := filepath.Clean(usr.HomeDir + "/.stx/config.stx.cue")
	if _, err := os.Stat(homeConfigPath); !os.IsNotExist(err) {
		log.Debug("Loading", homeConfigPath)
		buildArgs = append(buildArgs, homeConfigPath)
	}

	// look for config.stx.cue colocated with cue.mod
	localConfigPath := path + "/config.stx.cue"
	if _, err := os.Stat(localConfigPath); !os.IsNotExist(err) {
		log.Debug("Loading", localConfigPath)
		buildArgs = append(buildArgs, localConfigPath)
	}

	log.Debug("Building config...")
	buildInstances = GetBuildInstances(buildArgs, "stx")
	cueInstances = cue.Build(buildInstances)
	configInstance = cueInstances[0]
	configValue = configInstance.Value()

	configErr := configValue.Err()
	if configErr != nil {
		log.Fatal(configErr.Error())
	}

	cfg := Config{CueRoot: path, OsSeparator: separator}

	decodeErr := configValue.Decode(&cfg)
	if decodeErr != nil {
		log.Fatal("Config decode error", decodeErr.Error())
	}
	log.DebugF("Loaded config %+v", cfg)
	return &cfg
}
