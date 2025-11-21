package ssh

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	serverLifetime = 12 * time.Hour
)

// StartRequest defines the JSON structure for the start message.
type StartRequest struct {
	CaPublicKey    string `json:"ca_public_key"`
	HostPrivateKey string `json:"host_private_key"`
}

// sshServerController manages the state and lifecycle of the SSH server.
type sshServerController struct {
	mu           sync.Mutex
	config       *ssh.ServerConfig
	isRunning    bool
	listener     net.Listener
	shutdownChan chan struct{}
	sm           *sessionManager
	wg           sync.WaitGroup
}

func NewSshServerController() *sshServerController {
	return &sshServerController{
		sm: newSessionManager(),
	}
}

func (c *sshServerController) PrepareConfigAndListener(req StartRequest) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("SSH server is already running")
	}

	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.CaPublicKey))
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to parse CA public key: %w", err)
	}

	private, err := parsePrivateKey(req.HostPrivateKey)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to parse host private key: %w", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			_, ok := pubKey.(*ssh.Certificate)
			if !ok {
				return nil, fmt.Errorf("authentication failed: not a certificate")
			}
			certChecker := ssh.CertChecker{
				IsUserAuthority: func(auth ssh.PublicKey) bool {
					return bytes.Equal(auth.Marshal(), caPublicKey.Marshal())
				},
			}

			perms, err := certChecker.Authenticate(conn, pubKey)
			if err != nil {
				return nil, err
			}

			return perms, nil
		},
	}
	config.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to listen on 2222: %w", err)
	}

	c.config = config
	c.listener = listener
	c.isRunning = true
	c.shutdownChan = make(chan struct{})
	c.mu.Unlock()

	return nil
}

// Start initiates the SSH server in a new goroutine.
func (c *sshServerController) Start(req StartRequest) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("SSH server is already running")
	}

	// Prepare the configuration from the request payload
	caPublicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(req.CaPublicKey))
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to parse CA public key: %w", err)
	}

	private, err := parsePrivateKey(req.HostPrivateKey)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to parse host private key: %w", err)
	}

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			_, ok := pubKey.(*ssh.Certificate)
			if !ok {
				return nil, fmt.Errorf("authentication failed: not a certificate")
			}
			certChecker := ssh.CertChecker{
				IsUserAuthority: func(auth ssh.PublicKey) bool {
					return bytes.Equal(auth.Marshal(), caPublicKey.Marshal())
				},
			}

			perms, err := certChecker.Authenticate(conn, pubKey)
			if err != nil {
				return nil, err
			}

			return perms, nil
		},
	}
	config.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("failed to listen on 2222: %w", err)
	}

	c.listener = listener
	c.isRunning = true
	c.shutdownChan = make(chan struct{})
	c.mu.Unlock()

	log.Println("SSH server starting... Will automatically stop in", serverLifetime)

	//go c.runServer(config)

	return nil
}

// Run contains the main accept loop and the shutdown logic.
func (c *sshServerController) Run() {
	acceptDone := make(chan struct{})

	go func() {
		defer close(acceptDone)
		for {
			conn, err := c.listener.Accept()
			if err != nil {
				// This error is expected when the listener is closed.
				log.Printf("Listener stopped accepting connections: %v", err)
				return
			}
			c.wg.Add(1)
			go handleConnection(conn, c)
		}
	}()

	timer := time.NewTimer(serverLifetime)
	defer timer.Stop()

	shutdownMsg := ""
	gracePeriod := 3 * time.Second

	select {
	case <-c.shutdownChan:
		log.Println("Received stop command. Beginning graceful shutdown...")
		shutdownMsg = "Server shutdown initiated."
	case <-timer.C:
		log.Println("Server lifetime expired. Beginning graceful shutdown...")
		shutdownMsg = fmt.Sprintf("Server is shutting down after reaching its %s lifetime limit.", serverLifetime)
	}

	// exit accept loop goroutine
	c.mu.Lock()
	if c.listener != nil {
		c.listener.Close()
	}

	c.mu.Unlock()

	<-acceptDone
	log.Println("Accept loop has shut down.")

	fullShutdownMsg := fmt.Sprintf("!!! %s Shutting down in %s !!!", shutdownMsg, gracePeriod)
	c.sm.BroadcastAndClose(fullShutdownMsg, gracePeriod)
	log.Println("Grace period finished. All connections are closed.")

	log.Println("Waiting for all session cleanups to complete...")
	c.wg.Wait()

	c.mu.Lock()
	c.isRunning = false
	c.config = nil
	c.listener = nil
	c.shutdownChan = nil
	c.mu.Unlock()
	log.Println("SSH server has been shut down.")
}

func (c *sshServerController) Wait() {
	c.wg.Wait()
}

// Stop initiates a graceful shutdown of the SSH server.
func (c *sshServerController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		log.Println("Stop command received, but server is not running.")
		return
	}

	// Closing the channel signals the runServer goroutine to shut down
	close(c.shutdownChan)
}

func (c *sshServerController) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.isRunning
}
