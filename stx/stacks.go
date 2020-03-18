package stx

import (
	"errors"

	"cuelang.org/go/cue"
	"github.com/TangoGroup/stx/logger"
)

// Stack represents an individual stack
type Stack struct {
	Name, Profile, SopsProfile, Region, Environment, RegionCode string
	DependsOn                                                   []string
	Tags                                                        map[string]string
}

// Stacks represents the Go equivalent of the Cue Stacks pattern
type Stacks map[string]Stack

// GetStacks returns the (possibly filtered) stacks as decoded from the cue instance value,
func GetStacks(cueValue cue.Value, flags Flags) (Stacks, error) {

	stacks := make(Stacks)
	filteredStacks := make(Stacks)

	stacksCueValue := cueValue.Lookup("Stacks")
	if !stacksCueValue.Exists() {
		return nil, nil
	}

	decodeErr := stacksCueValue.Decode(&stacks)

	if decodeErr != nil {
		// evaluation errors (incomplete values, mismatched types, etc)
		return nil, decodeErr
	}

	if len(stacks) <= 0 {
		return nil, nil
	}

	log := logger.NewLogger(true, false)
	// apply global flags here so individual commands dont have to
	for stackName, stack := range stacks {

		if flags.Environment != "" && stack.Environment != flags.Environment {
			log.Debug("Filtering", stackName)
			continue
		}

		if flags.RegionCode != "" && stack.RegionCode != flags.RegionCode {
			log.Debug("Filtering", stackName)
			continue
		}

		if flags.Profile != "" && stack.Profile != flags.Profile {
			log.Debug("Filtering", stackName)
			continue
		}

		log.Debug("Adding", stackName)
		filteredStacks[stackName] = stack
	}

	return filteredStacks, nil
}

// StacksIterator is a wrapper around cue.Iterator that allows for filtering based on stack fields
type StacksIterator struct {
	cueIter cue.Iterator
	flags   Flags
}

// NewStacksIterator returns *StacksIterator
func NewStacksIterator(cueValue cue.Value, flags Flags) (*StacksIterator, error) {
	stacks := cueValue.Lookup("Stacks")
	if !stacks.Exists() {
		return nil, errors.New("Stacks is undefined")
	}

	fields, fieldsErr := stacks.Fields()
	if fieldsErr != nil {
		return nil, fieldsErr
	}

	return &StacksIterator{cueIter: fields, flags: flags}, nil
}

// Next moves the index forward and applies global filters. returns true if there is a value that passes the filters
func (it *StacksIterator) Next() bool {
	if !it.cueIter.Next() {
		return false
	}

	currentValue := it.cueIter.Value()
	// currentLabel, _ := currentValue.Label()

	// apply filters to the current value
	if it.flags.Environment != "" {
		environmentValue := currentValue.Lookup("Environment")
		if !environmentValue.Exists() {
			return it.Next()
		}
		environment, environmentErr := environmentValue.String()
		if environmentErr != nil {
			return it.Next()
		}
		if it.flags.Environment != environment {
			return it.Next()
		}
	}

	if it.flags.RegionCode != "" {
		regionCodeValue := currentValue.Lookup("RegionCode")
		if !regionCodeValue.Exists() {
			return it.Next()
		}
		regionCode, regionCodeErr := regionCodeValue.String()
		if regionCodeErr != nil {
			return it.Next()
		}
		if it.flags.RegionCode != regionCode {
			return it.Next()
		}
	}

	if it.flags.Profile != "" {
		profileValue := currentValue.Lookup("Profile")
		if !profileValue.Exists() {
			return it.Next()
		}
		profile, profileErr := profileValue.String()
		if profileErr != nil {
			return it.Next()
		}
		if it.flags.Profile != profile {
			return it.Next()
		}
	}

	return true
}

// Value returns the value from the cue.Iterator
func (it *StacksIterator) Value() cue.Value {
	return it.cueIter.Value()
}
