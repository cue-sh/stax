package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/format"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/spf13/cobra"
)

// saveCmd represents the save command
var saveCmd = &cobra.Command{
	Use:   "save",
	Short: "Saves stack outputs as importable libraries to cue.mod",
	Long: `save operates on every stack found in the evaluated cue files.
	
For each stack that has Outputs defined, save will query CloudFormation
and write the Outputs as cue-formatted key:value pairs. Each stack will be
saved as its own file with a .out.cue extension. 

By default the files will be stored in the same directory as the stack was defined,
but this can be overridden in config.stx.cue with:

Cmd: Save: OutFilePrefix: ""

For example if you want all your file viewer to group output files together you could
set the prefix to "z-" to get them grouped at the bottom. To get them grouped together
at the top, set the prefix to something like "0-" or "0ut-".

This string is completely arbitrary and up to you, and it supports directories if ending
with a "/".

NOTE: Cue will not load files that begin with underscore, so avoid setting prefix to "_"
or any string that begins with "_".
`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()
		stx.EnsureVaultSession(config)

		buildInstances := stx.GetBuildInstances(args, config.PackageName)

		stx.Process(buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := stx.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack stx.Stack
				decodeErr := stackValue.Decode(&stack)
				if decodeErr != nil {
					log.Error(decodeErr)
					continue
				}

				saveErr := saveStackOutputs(config, buildInstance, stack)
				if saveErr != nil {
					log.Error(saveErr)
				}
			}
		})
	},
}

func saveStackOutputs(config *stx.Config, buildInstance *build.Instance, stack stx.Stack) error {

	// get a session and cloudformation service client
	session := stx.GetSession(stack.Profile)
	cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))
	describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}
	describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)
	if describeStacksErr != nil {
		return describeStacksErr
	}

	if len(describeStacksOutput.Stacks[0].Outputs) < 1 {
		log.Infof("%s %s %s\n", au.White("Skipped"), au.Magenta(stack.Name), "with no outputs.")
		return nil
	}

	fileName := buildInstance.Dir + "/" + config.Cmd.Save.OutFilePrefix + stack.Name + ".out.cue"
	log.Infof("%s %s %s %s\n", au.White("Saving"), au.Magenta(stack.Name), au.White("⤏"), fileName)

	// create the .out.cue file
	result := "package outputs" + "\n\n\"" + stack.Name + "\": {\n"
	// convert cloudformation outputs into simple key:value pairs
	for _, output := range describeStacksOutput.Stacks[0].Outputs {
		result += fmt.Sprintf("\"%s\":\"%s\"\n", aws.StringValue(output.OutputKey), aws.StringValue(output.OutputValue))
	}
	result += "}\n"

	// use cue to format the output
	cueOutput, cueOutputErr := format.Source([]byte(result), format.Simplify())
	if cueOutputErr != nil {
		log.Debug(result)
		return cueOutputErr
	}

	// save it!
	if len(config.Cmd.Save.OutFilePrefix) > 0 && config.Cmd.Save.OutFilePrefix[len(config.Cmd.Save.OutFilePrefix)-1:] == config.OsSeparator {
		os.MkdirAll(buildInstance.Dir+"/"+config.Cmd.Save.OutFilePrefix, 0766)
	}

	writeErr := ioutil.WriteFile(fileName, []byte(cueOutput), 0644)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
