package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/Fyve-Labs/tessa-daemon/internal/remote_commands"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

/*
 * Local usage: ./tessad start -c config.yaml
 */
// startCmd represents the server command
var startCmd = &cobra.Command{
	Use:     "start",
	Aliases: []string{"serve", "server"},
	Short:   "A brief description of your command",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		conf, err := config.LoadConfig(cfgFile)
		if err != nil {
			slog.Error(fmt.Sprintf("loading config: %v", err))
			conf, err = tryFirstBootstrap(cmd)
			if err != nil {
				slog.Error("Waiting for device re-bootstrapping")
				for {
					time.Sleep(30 * time.Second)
					conf, err = config.LoadConfig(cfgFile)
					if err == nil {
						break
					}
					slog.Warn(fmt.Sprintf("config still not ready: %v. Retrying...", err))
				}
			}

			// reload config after bootstrap
			conf, _ = config.LoadConfig(cfgFile)
		}

		if err := startServer(conf); err != nil {
			slog.Error(fmt.Sprintf("start server: %v", err))
			os.Exit(1)
		}
	},
}

// tryFirstBootstrap Looks for bootstrap token placed in data_dir/token by the installation script
func tryFirstBootstrap(cmd *cobra.Command) (*config.Config, error) {
	deviceName, dataDir, serverUrl := getBootstrapOpts(cmd)
	if _, err := os.Stat(dataDir + "/token"); err != nil {
		return nil, err
	}

	slog.Info("Found bootstrap token. Trying to bootstrap device...")
	token, err := os.ReadFile(dataDir + "/token")
	if err != nil {
		return nil, err
	}

	cfg, err := bootstrap(deviceName, string(token), dataDir, serverUrl)
	if err != nil {
		slog.Error(fmt.Sprintf("bootstrap: %v", err))
		return nil, err
	}

	return cfg, nil
}

func startServer(conf *config.Config) error {
	nc, err := nats.Connect(conf.NatsUrl(), conf.NatsOptions()...)
	if err != nil {
		return err
	} else {
		slog.Info("Connected to NATS server", slog.String("url", nc.ConnectedUrl()))
	}

	tunnelManager, err := tunnel.NewManager(config.DeviceName, conf.TunnelConfig)
	if err != nil {
		return errors.Wrap(err, "create tunnel manager")
	}

	slog.Info("Listening for commands...")
	commandManager := remote_commands.NewCommandManager(nc, tunnelManager)
	if err = commandManager.Initialize(); err != nil {
		return errors.Wrap(err, "initialize Command Manager")
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	slog.Info("Shutting down...")
	_ = nc.Drain()

	if err = commandManager.Stop(); err != nil {
		slog.Error(fmt.Sprintf("stop Command Manager: %v", err))
	}

	tunnelManager.Stop()

	return nil
}

func init() {
	rootCmd.AddCommand(startCmd)

	applyBootstrapOpts(startCmd)
}
