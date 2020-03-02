package stx

import (
	"fmt"

	"cuelang.org/go/cue"
)

// Stack represents an individual stack
type Stack struct {
	Profile, SopsProfile, Region, Environment, RegionCode string
	Tags                                                  map[string]string
}

// Stacks represents the Go equivalent of the Cue Stacks pattern
type Stacks map[string]Stack

// GetStacks returns the stacks as decoded from the cue instance value
func GetStacks(cueValue cue.Value, flags Flags) Stacks {

	var stacks Stacks

	stacksCueValue := cueValue.Lookup("Stacks")
	if !stacksCueValue.Exists() {
		return nil
	}

	decodeErr := stacksCueValue.Decode(&stacks)
	if decodeErr != nil {
		// evaluation errors (incomplete values, mismatched types, etc)
		fmt.Println(decodeErr)
		return nil
	}

	if len(stacks) <= 0 {
		return nil
	}

	// apply global flags here so individual commands dont have to
	for stackName, stack := range stacks {
		if flags.Environment != "" && stack.Environment != flags.Environment {
			delete(stacks, stackName)
		}

		if flags.RegionCode != "" && stack.RegionCode != flags.RegionCode {
			delete(stacks, stackName)
		}

		if flags.Profile != "" && stack.Profile != flags.Profile {
			delete(stacks, stackName)
		}
	}

	return stacks
}
