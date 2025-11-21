package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Fyve-Labs/tessa-daemon/internal/ssh"
	"github.com/nats-io/nats.go"
)

const (
	natsStartSubject = "ssh.start.json"
	natsStopSubject  = "ssh.stop"
)

func main() {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = nats.DefaultURL
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	log.Println("Connected to NATS server at", nc.ConnectedUrl())

	controller := ssh.NewSshServerController()

	// Subscription for starting the server
	_, err = nc.Subscribe(natsStartSubject, func(m *nats.Msg) {
		log.Printf("Received SSH start request: %s", m.Subject)
		var req ssh.StartRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			log.Printf("ERROR: Could not unmarshal start request: %v", err)
			return
		}

		if err := controller.Start(req); err != nil {
			log.Printf("ERROR: Failed to start SSH server: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to start subject: %v", err)
	}

	// Subscription for stopping the server
	_, err = nc.Subscribe(natsStopSubject, func(m *nats.Msg) {
		log.Printf("Received stop request on subject '%s'", m.Subject)
		controller.Stop()
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to stop subject: %v", err)
	}

	log.Printf("Listening for commands on subjects '%s' and '%s'", natsStartSubject, natsStopSubject)

	// Keep the main application running
	// Wait for a termination signal to gracefully shut down the main app
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down NATS listener...")
	controller.Stop()

	log.Println("Waiting for server to shut down gracefully...")
	controller.Wait()

	log.Println("All processes finished. Exiting.")
}
