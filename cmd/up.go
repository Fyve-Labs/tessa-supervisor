package cmd

import (
	"fmt"
	"os"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	device "github.com/Fyve-Labs/tessa-daemon/internal/device"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const DefaultDataDir = "/etc/tessad"
const DefaultBoostrapServer = "https://device-api.fyve.dev"

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
		deviceName, dataDir, serverUrl := getBootstrapOpts(cmd)
		token, _ := cmd.Flags().GetString("token")

		_, err := bootstrap(deviceName, token, dataDir, serverUrl)
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(1)
		}
	},
}

func bootstrap(deviceName, token, dataDir, serverUrl string) (*config.Config, error) {
	if deviceName == "" {
		deviceName = device.Serial()
	}

	conf := &config.Config{
		DeviceName: deviceName,
		DataDir:    dataDir,
		TLS: &config.TLSConfig{
			CaFile:   fmt.Sprintf("%s/credentials/root.crt", dataDir),
			CertFile: fmt.Sprintf("%s/credentials/device.crt", dataDir),
			KeyFile:  fmt.Sprintf("%s/credentials/device.key", dataDir),
		},
	}

	err := device.Bootstrap(&device.BootstrapConfig{
		Subject: deviceName,
		Token:   token,
		CertDir: fmt.Sprintf("%s/credentials", dataDir),
		Url:     serverUrl,
	})

	if err != nil {
		return nil, err
	}

	fmt.Printf("Writing config to: %s\n", cfgFile)
	fmt.Println("--------")
	fmt.Println("")

	if err := writeConfig(conf); err != nil {
		return nil, fmt.Errorf("writing config: %v", err)
	}

	return conf, nil
}

func writeConfig(cfg *config.Config) error {
	yamlData, err := yaml.Marshal(&cfg)
	fmt.Println(string(yamlData))
	if err != nil {
		return err
	}

	return os.WriteFile(cfgFile, yamlData, 0600)
}

func applyBootstrapOpts(cmd *cobra.Command) {
	cmd.Flags().StringP("device-name", "n", "", "Device name")
	cmd.Flags().String("data", DefaultDataDir, "Directory to write device certificate and key")
	cmd.Flags().String("server", DefaultBoostrapServer, "Bootstrap server URL")
}

func getBootstrapOpts(cmd *cobra.Command) (string, string, string) {
	deviceName, _ := cmd.Flags().GetString("device-name")
	dataDir, _ := cmd.Flags().GetString("data")
	serverUrl, _ := cmd.Flags().GetString("server")

	return deviceName, dataDir, serverUrl
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)

	applyBootstrapOpts(bootstrapCmd)
	bootstrapCmd.Flags().StringP("token", "t", "", "Token produced by tessa-cli: tessa gen-token -n device-name")
	bootstrapCmd.Flags().BoolP("force", "f", false, "Force bootstrap even if device config already exists")
	_ = bootstrapCmd.MarkFlagRequired("device-name")
	_ = bootstrapCmd.MarkFlagRequired("token")
}
