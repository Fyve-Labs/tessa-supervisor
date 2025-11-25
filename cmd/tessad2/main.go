package tessad2

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/Fyve-Labs/tessa-daemon/internal/remote_commands"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
	"github.com/nats-io/nats.go"
)

/*
Test:

	# Generate SSH certificate
	step ssh certificate viet viet_ecdsa --no-agent --context prod --no-password --insecure --not-after 24h --provisioner oidc --force

	# Publish message
	tessa ssh device-name

	# Start server
	./tessad -data-dir data -device-name device-name
*/
func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	nc, err := newNatsClient(conf)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	} else {
		log.Println("Connected to NATS server at", nc.ConnectedUrl())
	}

	tunnelManager, err := tunnel.NewManager(*config.DeviceName, conf.TunnelConfig)
	if err != nil {
		log.Fatalf("Failed to create tunnel manager: %v", err)
	}

	commandManager := remote_commands.NewCommandManager(nc, tunnelManager)
	log.Println("Listening for commands...")
	if err = commandManager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize Remote Command Manager: %v", err)
	}

	// Keep the main application running
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	_ = nc.Drain()

	if err = commandManager.Stop(); err != nil {
		log.Printf("stop Remote Command Manager: %v \n", err)
	}

	tunnelManager.Stop()

	log.Println("All processes finished. Exiting.")
}

func newNatsClient(conf *config.Config) (*nats.Conn, error) {
	if conf.NatsServerConfig.TLSEnabled {
		return nats.Connect(
			fmt.Sprintf("tls://%s:%s", conf.NatsServerConfig.Hostname, conf.NatsServerConfig.Port),
			nats.RootCAs(conf.NatsServerConfig.TLSCaFile),
			nats.ClientCert(conf.NatsServerConfig.TLSCertFile, conf.NatsServerConfig.TLSKeyFile),
			nats.Timeout(10*time.Second),
		)
	}

	return nats.Connect(conf.NatsURL())
}
