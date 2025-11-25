package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"gopkg.in/yaml.v3"
)

const defaultNatsURL = "tls://52.7.199.211:4222"
const defaultTunnelAddr = "52.7.199.211"

var DeviceName string

type Config struct {
	DeviceName       string            `yaml:"deviceName"`
	DataDir          string            `yaml:"data"`
	NatsServerConfig *NatsServerConfig `yaml:"nats,omitempty"`
	TunnelConfig     *TunnelConfig     `yaml:"tunnel,omitempty"`
	TLS              *TLSConfig        `yaml:"tls"`
}

type NatsServerConfig struct {
	URL         *url.URL
	Hostname    string
	Port        string
	TLSEnabled  bool
	TLSCaFile   string
	TLSKeyFile  string
	TLSCertFile string
}

type TLSConfig struct {
	CaFile   string `yaml:"ca"`
	KeyFile  string `yaml:"key"`
	CertFile string `yaml:"cert"`
}

type TunnelConfig struct {
	ServerAddr  string
	TLSCaFile   string
	TLSKeyFile  string
	TLSCertFile string
}

func LoadConfig(cfgFile string) (*Config, error) {
	yamlFile, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, err
	}

	if config.DeviceName == "" {
		return nil, errors.New("device name was not set")
	}

	DeviceName = config.DeviceName
	if config.TLS == nil {
		return nil, errors.New("TLS credentials not found")
	}

	config.TunnelConfig = &TunnelConfig{
		ServerAddr:  config.TunnelAddr(),
		TLSCaFile:   config.TLS.CaFile,
		TLSKeyFile:  config.TLS.KeyFile,
		TLSCertFile: config.TLS.CertFile,
	}

	return &config, nil
}

func (c *Config) TunnelAddr() string {
	if val := os.Getenv("TESSA_TUNNEL_SERVER_ADDR"); val != "" {
		return val
	}

	return defaultTunnelAddr
}

func (c *Config) NatsUrl() string {
	if val := os.Getenv("TESSA_NATS_URL"); val != "" {
		return val
	}

	return defaultNatsURL
}

func (c *Config) NatsOptions() []nats.Option {
	opts := []nats.Option{
		nats.Timeout(30 * time.Second),
	}

	if strings.HasPrefix(c.NatsUrl(), "tls://") {
		opts = append(opts, nats.RootCAs(c.TLS.CaFile))
		opts = append(opts, nats.ClientCert(c.TLS.CertFile, c.TLS.KeyFile))
	}

	return opts
}
