package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if the config file is valid.",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented.")
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
