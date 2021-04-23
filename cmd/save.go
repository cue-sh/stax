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

To determine where these .out.cue files are saved, stx uses the path of the
stack's template.cfn.cue file relative to the cue root. If no template.cfn.cue
file is found, stx will use the path of the concrete leaf, relative to cue root.

As an example, consider the following tree:

infrastructure/
|-cue/                                      ("cue root")
| |-cue.mod/
| | |-usr/cfn.out/vpc/dev-vpc-usw2.out.cue  (outputs file)
| |-vpc/
| | |-template.cfn.cue                      (template)

Running stx save from infrastructure/cue/vpc/ will find the stack "dev-vpc-usw2"
defined in the template.cfn.cue file. stx will use vpc/ as the path relative to
cue root to create vpc/ as the path relative to cfn.out.

The outputs file in this example will declare its cue package as "vpc" since
that is the folder in which it is contained. Note that special characters such
as spaces or hyphens will be removed from folder and package names.
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
