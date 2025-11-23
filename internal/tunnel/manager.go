package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/fatedier/frp/client"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/pocketbase/pocketbase/tools/store"
)

type Manager struct {
	deviceName string
	frpc       *client.Service
	proxyCfgs  *store.Store[string, v1.ProxyConfigurer]
	cancel     context.CancelFunc
}

func NewManager(deviceName string, conf *config.TunnelConfig) (*Manager, error) {
	frpc, err := NewService(conf)
	if err != nil {
		return nil, err
	}

	return &Manager{
		deviceName: deviceName,
		frpc:       frpc,
		proxyCfgs:  store.New(map[string]v1.ProxyConfigurer{}),
	}, nil
}

func (m *Manager) ProxySSH(IP string, port int) {
	proxyCfg := &v1.TCPMuxProxyConfig{
		ProxyBaseConfig: v1.ProxyBaseConfig{
			Type: "tcpmux",
			Name: m.deviceName,
			ProxyBackend: v1.ProxyBackend{
				LocalIP:   IP,
				LocalPort: port,
			},
		},
		DomainConfig: v1.DomainConfig{
			CustomDomains: []string{m.deviceName},
		},
		Multiplexer: "httpconnect",
	}

	m.proxyCfgs.Set(net.JoinHostPort(IP, fmt.Sprintf("%d", port)), proxyCfg)
	if err := m.update(); err != nil {
		slog.Warn(err.Error())
	}
}

func (m *Manager) UnProxy(IP string, port int) {
	m.proxyCfgs.Remove(net.JoinHostPort(IP, fmt.Sprintf("%d", port)))
	if err := m.update(); err != nil {
		slog.Error(err.Error())
	}
}

func (m *Manager) update() error {
	reload := func() error {
		var proxyCfgs []v1.ProxyConfigurer
		for _, cfg := range m.proxyCfgs.GetAll() {
			proxyCfgs = append(proxyCfgs, cfg)
		}

		return m.frpc.UpdateAllConfigurer(proxyCfgs, nil)
	}

	// not started yet
	if m.cancel == nil {
		if m.proxyCfgs.Length() > 0 {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				if err := m.frpc.Run(ctx); err != nil {
					cancel()
					m.cancel = nil
				}
			}()

			m.cancel = cancel
			return reload()
		}

		return nil
	}

	// Already started, check if we need to restart
	if m.proxyCfgs.Length() == 0 {
		m.Stop()
		return nil
	}

	return reload()
}

func (m *Manager) Stop() {
	m.frpc.Close()
	m.cancel()
	m.cancel = nil
}
