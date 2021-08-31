package internal

import (
	"crypto/sha1"
	"fmt"
	"sort"

	"cuelang.org/go/cue"
	"cuelang.org/go/pkg/encoding/yaml"
)

// GetStackHash returns a hash of the stack used by change set names and state
// the hash is 2 hashes in 1: <stack>-<template>
// the stack portion is based on non-Template values that will necesitate a deployment
// the template portion is based solely on the template value
// separating the 2 allows you to gleen (via diffs) which portion contributed to deployment
func GetStackHash(stack Stack, stackValue cue.Value) (string, error) {

	// build a string out of values known to change between deployments
	stackString := stack.Profile + stack.Environment + stack.Region + stack.Role

	if len(stack.Overrides) > 0 {
		// sort the Overrides map[string]struct
		overridesKeys := getSortedOverrideKeys(stack.Overrides)
		for _, overrideKey := range overridesKeys {
			override := stack.Overrides[overrideKey]
			// sort the Overrides.Map map[string]string
			overrideMapKeys := getSortedMapKeys(override.Map)
			for _, mapKey := range overrideMapKeys {
				// append map key and value to stackString
				stackString = stackString + mapKey + override.Map[mapKey]
			}
		}
	}

	if len(stack.Tags) > 0 {
		// sort the tags
		tagsKeys := getSortedMapKeys(stack.Tags)
		for _, tagKey := range tagsKeys {
			stackString = stackString + tagKey + stack.Tags[tagKey]
		}
	}

	template := stackValue.LookupPath(cue.ParsePath("Template"))
	yml, ymlErr := yaml.Marshal(template)
	if ymlErr != nil {
		return "", ymlErr
	}

	return fmt.Sprintf("%x-%x", sha1.Sum([]byte(stackString)), sha1.Sum([]byte(yml))), nil
}

func getSortedOverrideKeys(m map[string]Override) []string {

	mk := make([]string, len(m))
	i := 0
	for k := range m {
		mk[i] = k
		i++
	}
	sort.Strings(mk)
	return mk
}

func getSortedMapKeys(m map[string]string) []string {

	mk := make([]string, len(m))
	i := 0
	for k := range m {
		mk[i] = k
		i++
	}
	sort.Strings(mk)
	return mk
}
