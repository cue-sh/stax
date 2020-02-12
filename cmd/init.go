package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go/aws"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/cue/parser"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

type stacks map[string]struct{ Profile, Region, Environment, RegionCode string }

type instanceHandler func(*build.Instance, *cue.Instance, cue.Value)

func loadCueInstances(args []string, handler instanceHandler) {
	const syntaxVersion = -1000 + 13

	config := load.Config{
		Package: "cfn",
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

func getStacks(cueValue cue.Value) stacks {
	// decoding into a struct allows Stacks to be indexed and used with for/range
	var stacks stacks
	stacksCueValue := cueValue.Lookup("Stacks")
	if stacksCueValue.Exists() {

		decodeErr := cueValue.Decode(&stacks)

		if decodeErr != nil {
			// evaluation errors (incomplete values, mismatched types, etc)
			fmt.Println(decodeErr)
		} else if len(stacks) > 0 {
			return stacks
		}
	}
	return nil
}

func ensureVaultSession() {
	sessionsOut, _ := exec.Command("aws-vault", "list", "--sessions").Output()
	//fmt.Println(string(sessionsOut))
	if len(sessionsOut) < 1 {
		fmt.Print("MFA: ")
		var input string
		fmt.Scanln(&input)
		err := exec.Command("aws-vault", "exec", "-t", input, "gloo-users").Run()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
}

type awsCredentials struct {
	AccessKeyId, SecretAccessKey, SessionToken, Expiration string
}

func getProfileCredentials(profile string) awsCredentials {
	execOut, execErr := exec.Command("aws-vault", "exec", "--json", profile).Output()

	if execErr != nil {
		fmt.Println(execErr)
		os.Exit(1)
	}
	// TODO: cache credentials until expired
	var credentials awsCredentials
	json.Unmarshal(execOut, &credentials)
	return credentials
}

func getSession(creds awsCredentials) *session.Session {
	config := aws.NewConfig().WithCredentials(credentials.NewStaticCredentials(creds.AccessKeyId, creds.SecretAccessKey, creds.SessionToken))
	sess, _ := session.NewSession(config)
	return sess
}
