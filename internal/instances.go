package internal

import (
	"path/filepath"
	"regexp"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/cue-sh/stax/logger"
)

type instanceHandler func(*build.Instance, *cue.Instance)

// GetBuildInstances loads and parses cue files and returns a list of build instances
func GetBuildInstances(args []string, pkg string) []*build.Instance {
	// const syntaxVersion = -1000 + 13

	config := load.Config{
		Package: pkg,
		Context: build.NewContext(
			build.ParseFile(func(name string, src interface{}) (*ast.File, error) {
				return parser.ParseFile(name, src,
					parser.FromVersion(parser.Latest),
					parser.ParseComments,
				)
			})),
	}
	if len(args) < 1 {
		args = append(args, "./...")
	}
	// load finds files based on args and passes those to build
	// buildInstances is a list of build.Instances, each has been parsed
	buildInstances := load.Instances(args, &config)

	return buildInstances
}

// Process iterates over instances, filters based on flags, and applies the handler function for each
func Process(config *Config, buildInstances []*build.Instance, flags Flags, log *logger.Logger, handler instanceHandler) {

	var excludeRegexp, includeRegexp *regexp.Regexp
	var excludeRegexpErr, includeRegexpErr error

	if flags.Exclude != "" {
		log.Debug("Compiling --exclude regexp...")
		excludeRegexp, excludeRegexpErr = regexp.Compile(flags.Exclude)
		if excludeRegexpErr != nil {
			log.Fatal(excludeRegexpErr)
		}
	}

	if flags.Include != "" {
		log.Debug("Compiling --include regexp...")
		includeRegexp, includeRegexpErr = regexp.Compile(flags.Include)
		if includeRegexpErr != nil {
			log.Fatal(includeRegexpErr)
		}
	}

	log.Debug("Iterating", len(buildInstances), "build instances...")
	for _, buildInstance := range buildInstances {

		// if user set save outfileprefix which ends with "/" (*nix) or "\" (win)
		// don't process that as a build instance
		outFilePrefix := config.Cmd.Save.OutFilePrefix
		if len(outFilePrefix) > 0 && outFilePrefix[len(outFilePrefix)-1:] == config.OsSeparator {
			outFolderName := outFilePrefix[:len(outFilePrefix)-1]
			if filepath.Base(buildInstance.Dir) == outFolderName {
				continue
			}
		}

		if excludeRegexp != nil && excludeRegexp.MatchString(buildInstance.DisplayPath) {
			log.Debug("Excluded via --exlude: ", buildInstance.DisplayPath)
			continue
		}

		if includeRegexp != nil && !includeRegexp.MatchString(buildInstance.DisplayPath) {
			log.Debug("NOT included via --include: ", buildInstance.DisplayPath)
			continue
		}
		// A cue instance defines a single configuration based on a collection of underlying CUE files.
		// cue.Build is designed to produce a single cue.Instance from n build.Instances
		// doing so however, would loose the connection between a stack and the build instance that
		// contains relevant path/file information related to the stack
		// here we cue.Build one at a time so we can maintain a 1:1:1:1 between
		// build.Instance, cue.Instance, cue.Value, and Stack
		cueInstance := cue.Build([]*build.Instance{buildInstance})[0]
		if cueInstance.Err != nil {
			// parse errors will be exposed here
			log.Error(cueInstance.Err, cueInstance.Err.Position())
			// TODO consider using log.Fatal and removing continue
			continue
		}

		log.Debug("Executing handler...")
		handler(buildInstance, cueInstance)
	}
}
