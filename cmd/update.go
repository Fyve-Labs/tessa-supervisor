package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update tessad CLI to latest version.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not yet implemented.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

}
