package client

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/smallstep/certificates/api"
)

type BootstrapConfig struct {
	Subject string
	Token   string
	CertDir string
	Url     string
}

func Bootstrap(c *BootstrapConfig) error {
	if err := os.MkdirAll(c.CertDir, 0700); err != nil {
		return err
	}

	privateKey, signReq, err := NewSignRequest(c.Subject)
	if err != nil {
		return err
	}

	body, err := json.Marshal(signReq)
	if err != nil {
		return err
	}

	client := &http.Client{}
	endpoint := strings.TrimRight(c.Url, "/") + "/install/request"

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Token", c.Token)
	req.Header.Set("User-Agent", "Tessa Client SDK 0.1")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		content, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("error response: %s (%d)", content, resp.StatusCode)
	}

	_ = privateKey
	var signResp api.SignResponse
	if err := json.NewDecoder(resp.Body).Decode(&signResp); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	// Encode server certificate with the intermediate
	chainPem, err := encodeX509(signResp.CertChainPEM...)
	if err != nil {
		return err
	}

	caPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signResp.CaPEM.Raw,
	})

	if err = os.WriteFile(c.CertDir+"/root.crt", caPem, 0644); err != nil {
		return err
	}

	if err = os.WriteFile(c.CertDir+"/device.crt", chainPem, 0644); err != nil {
		return err
	}

	keyPem, err := encodePrivateKey(privateKey)
	if err != nil {
		return err
	}

	if err = os.WriteFile(c.CertDir+"/device.key", keyPem, 0600); err != nil {
		return err
	}

	return nil
}

func encodePrivateKey(key crypto.PrivateKey) ([]byte, error) {
	b, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: b,
	}), nil
}

func encodeX509(certs ...api.Certificate) ([]byte, error) {
	certPem := bytes.NewBuffer([]byte{})
	for _, cert := range certs {
		err := pem.Encode(certPem, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		if err != nil {
			return nil, err
		}
	}

	return certPem.Bytes(), nil
}
