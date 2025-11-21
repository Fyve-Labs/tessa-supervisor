package config

import (
	"errors"
	"flag"
	"net/url"
	"os"
	"path/filepath"
)

const defaultDataDir = "/var/lib/tessa"
const defaultNatsURL = "nats://52.7.199.211:4222"

var (
	DeviceName  = flag.String("device-name", envOr("TESSA_DEVICE_NAME", ""), "Device name")
	dataDir     = flag.String("data-dir", envOr("TESSA_DATA_DIR", defaultDataDir), "Base data directory (credentials, etc.)")
	natsURL     = flag.String("nats-url", envOr("NATS_URL", defaultNatsURL), "Nats server URL")
	tlsCaFile   = flag.String("tls-ca", "", "TLS CA certificate file")
	tlsKeyFile  = flag.String("tls-key", "", "TLS private key file")
	tlsCertFile = flag.String("tls-cert", "", "TLS certificate file")
)

type Config struct {
	DataDir          string
	NatsServerConfig *NatsServerConfig
	TunnelConfig     *TunnelConfig
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

type TunnelConfig struct {
	ServerAddr  string
	TLSCaFile   string
	TLSKeyFile  string
	TLSCertFile string
}

func LoadConfig() (*Config, error) {
	flag.Parse()

	if DeviceName == nil || *DeviceName == "" {
		return nil, errors.New("device name was not set")
	}

	u, err := url.Parse(*natsURL)
	if err != nil {
		return nil, err
	}

	trySavedTLSFromDataDir()

	if *tlsCaFile == "" || *tlsKeyFile == "" || *tlsCertFile == "" {
		return nil, errors.New("TLS credentials not found")
	}

	natsServerConfig := &NatsServerConfig{
		URL:      u,
		Hostname: u.Hostname(),
		Port:     u.Port(),
	}

	natsServerConfig.TLSEnabled = true
	natsServerConfig.TLSCaFile = *tlsCaFile
	natsServerConfig.TLSKeyFile = *tlsKeyFile
	natsServerConfig.TLSCertFile = *tlsCertFile

	config := &Config{
		DataDir:          *dataDir,
		NatsServerConfig: natsServerConfig,
		TunnelConfig: &TunnelConfig{
			ServerAddr:  envOr("TESSA_TUNNEL_SERVER_ADDR", "52.7.199.211"),
			TLSCaFile:   *tlsCaFile,
			TLSKeyFile:  *tlsKeyFile,
			TLSCertFile: *tlsCertFile,
		},
	}

	return config, nil
}

func (c *Config) NatsURL() string {
	return c.NatsServerConfig.URL.String()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func trySavedTLSFromDataDir() {
	if *tlsCaFile != "" && *tlsKeyFile != "" && *tlsCertFile != "" {
		return
	}

	credDir := filepath.Join(*dataDir, "credentials")

	if file, err := os.Stat(filepath.Join(credDir, "certificate.crt")); err == nil && !file.IsDir() {
		*tlsCaFile = filepath.Join(credDir, "root_ca.crt")
		*tlsKeyFile = filepath.Join(credDir, "private.key")
		*tlsCertFile = filepath.Join(credDir, "certificate.crt")
	}
}
