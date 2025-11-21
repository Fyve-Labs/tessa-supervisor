package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/Fyve-Labs/tessa-daemon/internal/ssh"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
	"github.com/nats-io/nats.go"
)

const (
	natsStartSubject = "tessa.ssh.start.json"
	natsStopSubject  = "tessa.ssh.stop"
)

/*
*

  - Test:
    # Generate SSH certificate
    step ssh certificate viet viet_ecdsa --no-agent --context prod --no-password --insecure --not-after 24h --provisioner oidc --force

    # Publish message
    cat ssh.start.json | nats pub tessa.ssh.start.json --server tls://52.7.199.211:4222 --tlscert credentials/certificate.crt --tlskey credentials/private.key --tlsca credentials/root_ca.crt

    # Start server
    ./tessad -data-dir data -device-name macair
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

	var serverDoneChan chan struct{}
	controller := ssh.NewSshServerController()
	proxyContext, cancelProxy := context.WithCancel(context.Background())

	// Subscription
	startSub, err := nc.Subscribe(natsStartSubject, func(m *nats.Msg) {
		log.Printf("Received SSH start request: %s", m.Subject)
		var req ssh.StartRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			log.Printf("ERROR: Could not unmarshal start request: %v", err)
			return
		}

		if err := controller.PrepareConfigAndListener(req); err != nil {
			log.Printf("ERROR: Failed to start SSH server: %v", err)
		}

		serverDoneChan = make(chan struct{})
		go func() {
			controller.Run()
			close(serverDoneChan)
		}()

		proxySvc, err := tunnel.NewFrpService(*config.DeviceName, conf.TunnelConfig)
		if err != nil {
			log.Fatalf("Failed to create frp service: %v", err)
		}
		go func() {
			defer cancelProxy()
			proxySvc.Run(proxyContext)
		}()
	})

	if err != nil {
		log.Fatalf("Failed to subscribe to start subject: %v", err)
	}

	defer startSub.Drain()

	// Subscription for stopping the server
	stop, err := nc.Subscribe(natsStopSubject, func(m *nats.Msg) {
		log.Printf("Received stop request on subject '%s'", m.Subject)
		controller.Stop()
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to stop subject: %v", err)
	}

	defer stop.Drain()
	log.Printf("Listening for commands on subjects '%s' and '%s'", natsStartSubject, natsStopSubject)

	// Keep the main application running
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	_ = nc.Drain()
	cancelProxy()

	if controller.IsRunning() {
		controller.Stop()

		log.Println("Waiting for server to shut down gracefully...")
		<-serverDoneChan
	}

	log.Println("All processes finished. Exiting.")
}

func newNatsClient(conf *config.Config) (*nats.Conn, error) {
	if conf.NatsServerConfig.TLSEnabled {
		return nats.Connect(
			fmt.Sprintf("tls://%s:%s", conf.NatsServerConfig.Hostname, conf.NatsServerConfig.Port),
			nats.RootCAs(conf.NatsServerConfig.TLSCaFile),
			nats.ClientCert(conf.NatsServerConfig.TLSCertFile, conf.NatsServerConfig.TLSKeyFile),
		)
	}

	return nats.Connect(conf.NatsURL())
}
