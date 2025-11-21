package ssh

import (
	"encoding/base64"

	"golang.org/x/crypto/ssh"
)

func parsePrivateKey(privateKey string) (ssh.Signer, error) {
	if string(privateKey[:10]) == "-----BEGIN" {
		return ssh.ParsePrivateKey([]byte(privateKey))
	}

	keyBytes, err := base64.StdEncoding.DecodeString(privateKey)
	if err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(keyBytes)
}
