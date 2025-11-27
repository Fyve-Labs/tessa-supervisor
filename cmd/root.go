package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	VERSION   = "0.0.0"
	COMMIT    = "development"
	BUILDDATE = "unknown"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "tessad",
	Short: "Tessa Daemon is a daemon for managing Tessa devices.",

	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "/etc/tessad/config.yaml", "Config file")
}
