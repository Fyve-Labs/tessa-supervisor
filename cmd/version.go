package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version: %s-%s\n", VERSION, COMMIT)
		fmt.Printf("Build Date: %s", BUILDDATE)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
