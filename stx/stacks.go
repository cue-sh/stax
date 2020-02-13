package stx

import (
	"fmt"

	"cuelang.org/go/cue"
)

// Stack represents an individual stack
type Stack struct{ Profile, Region, Environment, RegionCode string }

// Stacks represents the Go equivalent of the Cue Stacks pattern
// it allows stacks to be indexed via for range
type Stacks map[string]Stack

// GetStacks returns the stacks as decoded from the cue instance value
func GetStacks(cueValue cue.Value) Stacks {
	// decoding into a struct allows Stacks to be indexed and used with for/range
	var stacks Stacks
	stacksCueValue := cueValue.Lookup("Stacks")
	if stacksCueValue.Exists() {

		decodeErr := stacksCueValue.Decode(&stacks)

		if decodeErr != nil {
			// evaluation errors (incomplete values, mismatched types, etc)
			fmt.Println(decodeErr)
		} else if len(stacks) > 0 {
			return stacks
		}
	}
	return nil
}
