package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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
	Long:  `Yada Yada Yada...`,
	Run: func(cmd *cobra.Command, args []string) {

		defer log.Flush()
		stx.EnsureVaultSession(config)

		buildInstances := stx.GetBuildInstances(args, "cfn")

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

				saveErr := saveStackOutputs(buildInstance, stack)
				if saveErr != nil {
					log.Error(saveErr)
				}
			}
		})
	},
}

func saveStackOutputs(buildInstance *build.Instance, stack stx.Stack) error {

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

	// cfn.out files are store under cue.mod/usr/cfn.out with the same relative path as the stacks cue instance
	// for example a stack with a template declared in cue/engineering/eks/cluster
	// with a concrete leaf in cue/engineering/eks/cluster/dev-usw2
	// would store outputs in cue.mod/usr/cfn.out/cue/engineering/eks/cluster
	instancePath := buildInstance.Dir
	// in case no template.cfn.cue file is found, use the instance (relative) path
	cueOutPath := strings.Replace(instancePath, buildInstance.Root, "", 1)

	// look for the template.cfn.cue file for the current build instance
	dirs := strings.Split(instancePath, config.OsSeparator)
	path := ""
	// traverse the directory tree starting from leaf going up to successive parents
	for i := len(dirs); i > 0; i-- {
		path = strings.Join(dirs[:i], config.OsSeparator)
		// look for the template file
		if _, err := os.Stat(path + config.OsSeparator + "template.cfn.cue"); !os.IsNotExist(err) {
			break // found it!
		}
	}
	if path != "" {
		cueOutPath = strings.Replace(path, buildInstance.Root, "", 1)
	}
	cueOutPath = strings.Replace(buildInstance.Root+"/cue.mod/usr/cfn.out"+cueOutPath, "-", "", -1)
	fileName := cueOutPath + "/" + stack.Name + ".out.cue"
	log.Infof("%s %s %s %s\n", au.White("Saving"), au.Magenta(stack.Name), au.White("⤏"), fileName)

	// create the .out.cue file
	cuePackage := filepath.Base(cueOutPath)
	result := "package " + cuePackage + "\n\n\"" + stack.Name + "\": {\n"
	// convert cloudformation outputs into simple key:value pairs
	for _, output := range describeStacksOutput.Stacks[0].Outputs {
		result += fmt.Sprintf("\"%s\":\"%s\"\n", aws.StringValue(output.OutputKey), aws.StringValue(output.OutputValue))
	}
	result += "}\n"

	// use cue to format the output
	cueOutput, cueOutputErr := format.Source([]byte(result))
	if cueOutputErr != nil {
		log.Debug(result)
		return cueOutputErr
	}

	// save it!
	os.MkdirAll(cueOutPath, 0766)
	writeErr := ioutil.WriteFile(fileName, cueOutput, 0766)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
