package tunnel

import (
	"github.com/Fyve-Labs/tessa-daemon/internal/config"
	"github.com/fatedier/frp/client"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
)

func NewService(conf *config.TunnelConfig) (*client.Service, error) {
	enabled := true
	clientCfg := &v1.ClientCommonConfig{
		ServerAddr: "52.7.199.211",
		ServerPort: 7000,
		Transport: v1.ClientTransportConfig{
			Protocol: "tcp",
			TLS: v1.TLSClientConfig{
				Enable: &enabled,
				TLSConfig: v1.TLSConfig{
					CertFile:      conf.TLSCertFile,
					KeyFile:       conf.TLSKeyFile,
					TrustedCaFile: conf.TLSCaFile,
				},
			},
		},
	}

	if err := clientCfg.Complete(); err != nil {
		return nil, err
	}

	if _, err := validation.ValidateClientCommonConfig(clientCfg); err != nil {
		return nil, err
	}

	//proxyCfgs.Complete(clientCfg.User)
	//if err := validation.ValidateProxyConfigurerForClient(proxyCfgs); err != nil {
	//	return nil, err
	//}

	return client.NewService(client.ServiceOptions{
		Common:         clientCfg,
		ProxyCfgs:      []v1.ProxyConfigurer{},
		VisitorCfgs:    nil,
		ConfigFilePath: "",
	})
}
