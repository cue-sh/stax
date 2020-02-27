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
		stx.EnsureVaultSession(config)
		buildInstances := stx.GetBuildInstances(args, "cfn")
		stx.Process(buildInstances, flags, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := stx.GetStacks(cueValue, flags)
			if stacks != nil {
				for stackName, stack := range stacks {
					// get a session and cloudformation service client
					session := stx.GetSession(stack.Profile)
					cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))
					describeStacksInput := cloudformation.DescribeStacksInput{StackName: &stackName}
					describeStacksOutput, describeStacksErr := cfn.DescribeStacks(&describeStacksInput)
					if describeStacksErr != nil {
						fmt.Println(au.Red(describeStacksErr))
						continue
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
					cueOutPath = buildInstance.Root + "/cue.mod/usr/cfn.out" + cueOutPath

					// create the .out.cue file
					cuePackage := filepath.Base(cueOutPath)
					result := "package " + cuePackage + "\n\n\"" + stackName + "\": {\n"
					// convert cloudformation outputs into simple key:value pairs
					for _, output := range describeStacksOutput.Stacks[0].Outputs {
						result += fmt.Sprintf("\"%s\":\"%s\"\n", *output.OutputKey, *output.OutputValue)
					}
					result += "}\n"
					// use cue to format the output
					cueOutput, cueOutputErr := format.Source([]byte(result))
					if cueOutputErr != nil {
						fmt.Println(au.Red(cueOutputErr))
						continue
					}
					os.MkdirAll(cueOutPath, 0766)
					fileName := cueOutPath + "/" + stackName + ".out.cue"
					ioutil.WriteFile(fileName, cueOutput, 0766)
					fmt.Printf("%s %s %s %s\n", au.White("Saved"), au.Magenta(stackName), au.White("⤏"), fileName)
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(saveCmd)
}
