package stx

import (
	"errors"
	"regexp"
	"strings"

	"cuelang.org/go/cue"
	"github.com/TangoGroup/stx/logger"
)

// Stack represents the decoded value of stacks[stackname]
type Stack struct {
	Name, Profile, Region, Environment, RegionCode string
	Overrides                                      map[string]struct {
		SopsProfile string
		Map         map[string]string
	}
	DependsOn   []string
	Tags        map[string]string
	TagsEnabled bool
}

// StacksIterator is a wrapper around cue.Iterator that allows for filtering based on stack fields
type StacksIterator struct {
	cueIter cue.Iterator
	flags   Flags
	log     *logger.Logger
}

// NewStacksIterator returns *StacksIterator
func NewStacksIterator(cueInstance *cue.Instance, flags Flags, log *logger.Logger) (*StacksIterator, error) {
	log.Debug("Getting stacks...")
	stacks := cueInstance.Value().Lookup("Stacks")
	if !stacks.Exists() {
		return nil, errors.New("Stacks is undefined")
	}

	fields, fieldsErr := stacks.Fields()
	if fieldsErr != nil {
		return nil, fieldsErr
	}

	return &StacksIterator{cueIter: fields, flags: flags, log: log}, nil
}

// Next moves the index forward and applies global filters. returns true if there is a value that passes the filters
func (it *StacksIterator) Next() bool {
	if !it.cueIter.Next() {
		return false
	}

	currentValue := it.cueIter.Value()
	if it.flags.StackNameRegexPattern != "" {
		stackName, _ := currentValue.Label()
		var stackNameRegexp *regexp.Regexp
		var stackNameRegexpErr error

		it.log.Debug("Compiling --stacks regexp...")
		stackNameRegexp, stackNameRegexpErr = regexp.Compile(it.flags.StackNameRegexPattern)
		if stackNameRegexpErr != nil {
			it.log.Fatal(stackNameRegexpErr)
		}
		if !stackNameRegexp.MatchString(stackName) {
			return it.Next()
		}
	}

	// apply filters to the current value
	if it.flags.Environment != "" {
		environmentValue := currentValue.Lookup("Environment")
		if !environmentValue.Exists() {
			return it.Next()
		}
		environment, environmentErr := environmentValue.String()
		if environmentErr != nil {
			it.log.Error(environmentErr)
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
			it.log.Error(regionCodeErr)
			return it.Next()
		}
		if it.flags.RegionCode != regionCode {
			return it.Next()
		}
	}

	if it.flags.Profile != "" {
		it.log.Debug("Evaluating --profile", it.flags.Profile)
		profileValue := currentValue.Lookup("Profile")
		if !profileValue.Exists() {
			return it.Next()
		}
		profile, profileErr := profileValue.String()
		if profileErr != nil {
			it.log.Error(profileErr)
			return it.Next()
		}
		if it.flags.Profile != profile {
			return it.Next()
		}
	}

	if it.flags.Has != "" {
		it.log.Debug("Evaluating --has", it.flags.Has)
		path := strings.Split(it.flags.Has, ".")
		hasValue := currentValue.Lookup(path...)
		if !hasValue.Exists() {
			return it.Next()
		}
	}

	return true
}

// Value returns the value from the cue.Iterator
func (it *StacksIterator) Value() cue.Value {
	return it.cueIter.Value()
}
