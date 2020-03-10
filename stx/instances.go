package stx

import (
	"regexp"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/TangoGroup/stx/logger"
)

type instanceHandler func(*build.Instance, *cue.Instance, cue.Value)

// GetBuildInstances loads and parses cue files and returns a list of build instances
func GetBuildInstances(args []string, pkg string) []*build.Instance {
	const syntaxVersion = -1000 + 13

	config := load.Config{
		Package: pkg,
		Context: build.NewContext(
			build.ParseFile(func(name string, src interface{}) (*ast.File, error) {
				return parser.ParseFile(name, src,
					parser.FromVersion(syntaxVersion),
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
func Process(buildInstances []*build.Instance, flags Flags, handler instanceHandler) {

	log := logger.NewLogger(flags.Debug, flags.NoColor).WithPrefix("Process: ")

	var excludeRegexp, includeRegexp *regexp.Regexp
	var excludeRegexpErr, includeRegexpErr error

	if flags.Exclude != "" {
		excludeRegexp, excludeRegexpErr = regexp.Compile(flags.Exclude)
		if excludeRegexpErr != nil {
			log.Error(excludeRegexpErr.Error())
		}
	}

	if flags.Include != "" {
		includeRegexp, includeRegexpErr = regexp.Compile(flags.Include)
		if includeRegexpErr != nil {
			log.Error(includeRegexpErr.Error())
		}
	}

	for _, buildInstance := range buildInstances {
		if excludeRegexp != nil && excludeRegexp.MatchString(buildInstance.DisplayPath) {
			continue
		}

		if includeRegexp != nil && !includeRegexp.MatchString(buildInstance.DisplayPath) {
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
			continue
		}
		handler(buildInstance, cueInstance, cueInstance.Value())
	}
}
