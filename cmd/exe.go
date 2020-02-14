package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// exeCmd represents the exe command
var exeCmd = &cobra.Command{
	Use:   "exe",
	Short: "EXEcute a changeset for the evaluted leaves.",
	Long:  `Yada yada yada.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("exe called")
	},
}

func init() {
	rootCmd.AddCommand(exeCmd)

	// TODO add a flag to watch events
}
