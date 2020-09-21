package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/TangoGroup/stx/stx"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(deleteCmd)
}

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Deletes the stack along with .yml and .out.cue files",
	Long: `delete will operate on every stack found among the evaluated cue files.

For each stack, delete will—as the name suggests—DELETE the stack!

Beware that the only safety mechanism provided is a requirement to enter the
stack name (case-sensitive match).

** It your responsibility to ensure the proper authorization policies are 
applied to the credentials being used! **
`,
	Run: func(cmd *cobra.Command, args []string) {

		//TODO add debug messages
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
				log.Infof("%s %s %s %s:%s %s\n", au.Red("You are about to DELETE"), au.Magenta(stack.Name), au.Red("from"), au.Green(stack.Profile), au.Cyan(stack.Region), au.Red("."))
				log.Infof("%s\n%s\n%s", au.Index(255-88, "Are you sure you want to DELETE this stack?"), au.Gray(11, "Enter the name of the stack to confirm."), au.Gray(11, "▶︎"))
				var input string
				fmt.Scanln(&input)

				if input != stack.Name {
					continue
				}

				session := stx.GetSession(stack.Profile)
				cfn := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))

				log.Infof("%s %s %s %s:%s\n", au.White("Deleting"), au.Magenta(stack.Name), au.White("⤎"), au.Green(stack.Profile), au.Cyan(stack.Region))
				deleteStackInput := cloudformation.DeleteStackInput{StackName: aws.String(stack.Name)}
				_, deleteStackErr := cfn.DeleteStack(&deleteStackInput)
				if deleteStackErr != nil {
					log.Error(deleteStackErr)
					continue
				}

				// TODO DRY this out! It repeats in save.go
				instancePath := buildInstance.Dir
				cueOutPath := strings.Replace(instancePath, buildInstance.Root, "", 1)
				dirs := strings.Split(instancePath, config.OsSeparator)
				path := ""
				for i := len(dirs); i > 0; i-- {
					path = strings.Join(dirs[:i], config.OsSeparator)
					if _, err := os.Stat(path + config.OsSeparator + "template.cfn.cue"); !os.IsNotExist(err) {
						break // found it!
					}
				}
				if path != "" {
					cueOutPath = strings.Replace(path, buildInstance.Root, "", 1)
				}
				cueOutPath = strings.Replace(buildInstance.Root+"/cue.mod/usr/cfn.out"+cueOutPath, "-", "", -1)
				outputsFileName := cueOutPath + "/" + stack.Name + ".out.cue"

				if _, deleteOutputsErr := os.Stat(outputsFileName); deleteOutputsErr == nil {
					deleteOutputsErr := os.Remove(outputsFileName)
					if deleteOutputsErr != nil {
						log.Error(deleteOutputsErr)
					} else {
						log.Infof("%s %s\n", au.White("Removed →"), au.Gray(11, outputsFileName))
					}
				} else {
					log.Check()
				}

				dir := filepath.Clean(config.CueRoot + "/" + config.Cmd.Export.YmlPath + "/" + stack.Profile)
				cfnFileName := dir + "/" + stack.Name + ".cfn.yml"

				if _, deleteCfnErr := os.Stat(cfnFileName); deleteCfnErr == nil {
					deleteCfnErr := os.Remove(cfnFileName)
					if deleteCfnErr != nil {
						log.Error(deleteCfnErr)
					} else {
						log.Infof("%s %s\n", au.White("Removed →"), au.Gray(11, cfnFileName))
					}
				} else {
					log.Check()
				}
			}
		})
	},
}
