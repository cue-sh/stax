package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

// dplCmd represents the dpl command
var dplCmd = &cobra.Command{
	Use:   "dpl",
	Short: "DePLoys a stack by creating a changeset and previews expected changes.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {
		ensureVaultSession()

		loadCueInstances(args, func(buildInstance *build.Instance, cueInstance *cue.Instance, cueValue cue.Value) {
			stacks := getStacks(cueValue)
			if stacks != nil {
				//fmt.Printf("%+v\n\n", top)
				au := aurora.NewAurora(true)
				for stackName, stack := range stacks {
					dir := filepath.Clean(buildInstance.Root + "/../yml/cfn/" + stack.Profile)
					fileName := dir + "/" + stackName + ".cfn.yml"

					fmt.Printf("%s %s %s %s:%s\n", au.White("Deploying"), au.Magenta(stackName), au.White("⤏"), au.Green(stack.Profile), au.Brown(stack.Region))

					//TODO: reduce this to a single call to getSesession(stack.Profile). getSession should then call getProfileCredentials
					credentials := getProfileCredentials(stack.Profile)
					session := getSession(credentials)

					templateFileBytes, _ := ioutil.ReadFile(fileName)
					templateBody := string(templateFileBytes)

					svc := cloudformation.New(session, aws.NewConfig().WithRegion(stack.Region))
					validateTemplateInput := cloudformation.ValidateTemplateInput{
						TemplateBody: &templateBody,
					}
					validateTemplateOutput, validateTemplateErr := svc.ValidateTemplate(&validateTemplateInput)
					fmt.Printf("%+v\n", validateTemplateOutput)
					fmt.Printf("%+v\n", validateTemplateErr)
				}
			}
		})
	},
}

func init() {
	rootCmd.AddCommand(dplCmd)
}
