package tunnel

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"

	chclient "github.com/jpillora/chisel/client"
)

// Manager coordinates the lifecycle of a tunnel client configuration.
// It can only start when server, remotes and TLS are configured.
type Manager struct {
	server  string
	remotes []string
	tls     chclient.TLSConfig

	// flags to confirm explicit configuration
	setTLS bool

	// runtime
	mu      sync.Mutex
	client  *chclient.Client
	started bool
	ctx     context.Context
	runCtx  context.Context
	cancel  context.CancelFunc
	logger  *log.Logger
}

func NewManager(ctx context.Context, logger *log.Logger) *Manager {
	return &Manager{
		ctx:    ctx,
		logger: logger,
	}
}

// Ready reports whether the manager has sufficient configuration to start.
func (m *Manager) Ready() bool {
	return m.server != "" && len(m.remotes) > 0 && m.setTLS
}

// SetServer sets the tunnel server address.
func (m *Manager) SetServer(server string) {
	if server != m.server {
		m.server = server
	}
}

func (m *Manager) SetRemotes(remotes []string) {
	m.remotes = append([]string(nil), remotes...)
}

// AddRemote appends a remote and restarts if running.
func (m *Manager) AddRemote(remote string) {
	if slices.Contains(m.remotes, remote) {
		return
	}

	m.remotes = append(m.remotes, remote)
}

// SetTLS configures TLS.
func (m *Manager) SetTLS(tls chclient.TLSConfig) {
	m.tls = tls
	m.setTLS = true
}

func (m *Manager) Start() error {
	if m.started {
		return nil
	}

	if !m.Ready() {
		return errors.New("tunnel manager not ready")
	}

	m.logger.Printf("Starting tunnel client")
	m.mu.Lock()
	defer m.mu.Unlock()

	// derive a cancelable context for this run and snapshot config
	runCtx, cancel := context.WithCancel(m.ctx)
	server := m.server
	remotes := append([]string(nil), m.remotes...)
	tls := m.tls
	m.runCtx = runCtx
	m.cancel = cancel
	m.started = true

	// start the tunnel client and does not block
	c, err := Start(runCtx, &Config{Server: server, Remotes: remotes, TLS: tls})
	if err != nil {
		m.started = false
		return err
	}

	m.client = c

	return nil
}

func (m *Manager) Stop() error {
	if !m.started {
		return nil
	}

	m.mu.Lock()
	m.logger.Printf("Stoping tunnel client")
	if m.cancel != nil {
		m.cancel()
	}

	m.started = false
	m.client = nil
	m.runCtx = nil
	m.cancel = nil
	m.mu.Unlock()

	return nil
}

func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}

	return m.Start()
}
