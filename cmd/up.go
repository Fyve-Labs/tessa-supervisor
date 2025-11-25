package cmd

import (
	"fmt"
	"os"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	device "github.com/Fyve-Labs/tessa-daemon/internal/device"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

/*
 * Bootstrap device using token.
 * Local usage: ./tessad up --data testdata --token 6J2kM875DKIiYlKsd50LJ8iC1S33LL8ELfLckEyBrBe4257fnOJF1hk2I75UVz52WWRdv -c config.yaml -n rpi3 --force
 */
// bootstrapCmd represents the bootstrap command
var bootstrapCmd = &cobra.Command{
	Use:     "up",
	Aliases: []string{"bootstrap"},
	Short:   "Bootstrap Tessa device using token.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")
		deviceName, _ := cmd.Flags().GetString("device-name")

		if deviceName == "" {
			return fmt.Errorf("device name is required")
		}

		if token == "" {
			return fmt.Errorf("token is required")
		}

		if _, err := os.Stat(cfgFile); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return fmt.Errorf("Config file already exists at %s. Use --force to continue.\n", cfgFile)
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		deviceName, _ := cmd.Flags().GetString("device-name")
		baseDir, _ := cmd.Flags().GetString("data")
		token, _ := cmd.Flags().GetString("token")
		serverUrl, _ := cmd.Flags().GetString("server-url")

		conf := &config.Config{
			DeviceName: deviceName,
			DataDir:    baseDir,
			TLS: &config.TLSConfig{
				CaFile:   fmt.Sprintf("%s/credentials/root.crt", baseDir),
				CertFile: fmt.Sprintf("%s/credentials/device.crt", baseDir),
				KeyFile:  fmt.Sprintf("%s/credentials/device.key", baseDir),
			},
		}

		err := device.Bootstrap(&device.BootstrapConfig{
			Subject: deviceName,
			Token:   token,
			CertDir: fmt.Sprintf("%s/credentials", baseDir),
			Url:     serverUrl,
		})

		if err != nil {
			fmt.Printf("Error bootstrapping device: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Writing config to: %s\n", cfgFile)
		fmt.Println("--------")
		fmt.Println("")

		if err := writeConfig(conf); err != nil {
			fmt.Printf("Error writing config: %v\n", err)
			os.Exit(1)
		}
	},
}

func writeConfig(cfg *config.Config) error {
	yamlData, err := yaml.Marshal(&cfg)
	fmt.Println(string(yamlData))
	if err != nil {
		return err
	}

	return os.WriteFile(cfgFile, yamlData, 0600)
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)

	bootstrapCmd.Flags().StringP("device-name", "n", "", "Device name")
	bootstrapCmd.Flags().StringP("token", "t", "", "Token produced by tessa-cli: tessa gen-token -n device-name")
	bootstrapCmd.Flags().String("data", "/etc/tessad", "Directory to write device certificate and key")
	bootstrapCmd.Flags().String("server-url", "https://device-api.fyve.dev", "Bootstrap server URL")
	bootstrapCmd.Flags().BoolP("force", "f", false, "Force bootstrap even if device config already exists")
	_ = bootstrapCmd.MarkFlagRequired("device-name")
	_ = bootstrapCmd.MarkFlagRequired("token")
}
