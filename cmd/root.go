package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

//Notes: ALWAYS Retrieve Values from Viper
//This is the most critical step. Inside your command’s Run or RunE function, you must always retrieve configuration values using the Viper getter methods (e.g., viper.GetString("my-flag"), viper.GetBool("some-toggle")).
//
//Do not read from the original variable tied to the flag. The Viper instance is the single source of truth that has already resolved the final value based on its precedence rules (flag > env > config > default)

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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "/etc/tessa/config.yaml", "Config file")
}
