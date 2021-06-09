package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/format"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/cue-sh/stax/internal"
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

The outputs themselves do not need to be defined in Cue. For example if you
want to reference outputs in stacks that were deployed with some other tool,
all you need is the stack name, profile, and region to be defined in Cue.
From there stax will pull outputs from the CloudFormation API.

By default the output files will be stored in the same directory as the stack was
defined, but this can be overridden in config.stax.cue via:

Cmd: Save: OutFilePrefix: ""

This string is completely arbitrary, and it supports directories if ending with a "/".

For example if you want your file viewer to group output files together, you could
set the prefix to "z-" to get them grouped at the bottom. To get them grouped together
at the top, set the prefix to something like "0-" or "0ut-" as examples. To put them
in a subfolder set the prefix to something like "outputs/"

NOTE: Cue will not load files that begin with underscore, so avoid setting prefix to "_"
or any string that begins with "_".

`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()

		buildInstances := internal.GetBuildInstances(args, config.PackageName)

		internal.Process(config, buildInstances, flags, log, func(buildInstance *build.Instance, cueInstance *cue.Instance) {
			stacksIterator, stacksIteratorErr := internal.NewStacksIterator(cueInstance, flags, log)
			if stacksIteratorErr != nil {
				log.Fatal(stacksIteratorErr)
			}

			for stacksIterator.Next() {
				stackValue := stacksIterator.Value()
				var stack internal.Stack
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

func saveStackOutputs(config *internal.Config, buildInstance *build.Instance, stack internal.Stack) error {

	// get a session and cloudformation service client
	cfn := internal.GetCloudFormationClient(stack.Profile, stack.Region)

	describeStacksInput := cloudformation.DescribeStacksInput{StackName: aws.String(stack.Name)}
	describeStacksOutput, describeStacksErr := cfn.DescribeStacks(context.TODO(), &describeStacksInput)
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
		result += fmt.Sprintf("\"%s\":\"%s\"\n", aws.ToString(output.OutputKey), aws.ToString(output.OutputValue))
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
