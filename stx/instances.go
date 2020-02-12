package stx

import (
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
)

type instanceHandler func(*build.Instance, *cue.Instance, cue.Value)

// LoadCueInstances loads the cue files in the specified pkg, with the args (eg: ./test.cue or ./...)
func LoadCueInstances(args []string, pkg string, handler instanceHandler) {
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

	for _, buildInstance := range buildInstances {
		// A cue instance defines a single configuration based on a collection of underlying CUE files.
		// cue.Build is designed to produce a single cue.Instance from n build.Instances
		// doing so however, would loose the connection between a stack and the build instance that
		// contains relevant path/file information related to the stack
		// here we cue.Build one at a time so we can maintain a 1:1:1:1 between
		// build.Instance, cue.Instance, cue.Value, and Stack
		cueInstance := cue.Build([]*build.Instance{buildInstance})[0]
		if cueInstance.Err != nil {
			// parse errors will be exposed here
			fmt.Println(cueInstance.Err, cueInstance.Err.Position())
		} else {

			cueValue := cueInstance.Value()

			handler(buildInstance, cueInstance, cueValue)

		}
	}
}
