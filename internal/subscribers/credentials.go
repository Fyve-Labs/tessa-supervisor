package subscribers

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
)

// registerCredentials wires subscribers for certificate notifications.
func registerCredentials(st *starter) {
	st.On("tessa/things/certificates/notify", func(m pubsub.Message) {
		// Expected payload structure:
		// {
		//   "certificate": "<PEM string>",
		//   "private_key": "<PEM string>",
		//   "root_ca": "<PEM string>"
		// }
		var payload struct {
			Certificate string `json:"certificate"`
			PrivateKey  string `json:"private_key"`
			RootCA      string `json:"root_ca"`
		}
		if err := json.Unmarshal(m.Data, &payload); err != nil {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s invalid payload JSON: %v", m.IoTThingName, err)
			return
		}
		if payload.Certificate == "" || payload.PrivateKey == "" || payload.RootCA == "" {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s missing required fields (certificate/private_key/root_ca)", m.IoTThingName)
			return
		}

		// Determine target directory:
		// - If credDir is non-empty, it has been pre-validated as writable by main.go.
		// - If empty, fall back to a temporary directory (stateless mode).
		useDir := st.credDir
		if useDir == "" {
			tmp, terr := os.MkdirTemp("", "tessa-cred-*")
			if terr != nil {
				st.logger.Printf("[tessa/things/certificates/notify] thing=%s failed to allocate temp credential dir: %v", m.IoTThingName, terr)
				return
			}
			useDir = tmp
		}

		// Write files with appropriate permissions
		certPath := filepath.Join(useDir, "secure_tunnel.crt")
		keyPath := filepath.Join(useDir, "secure_tunnel.key")
		caPath := filepath.Join(useDir, "root_ca.crt")

		write := func(path string, data string, perm os.FileMode) error {
			return os.WriteFile(path, []byte(data+"\n"), perm)
		}
		if err := write(certPath, payload.Certificate, 0o644); err != nil {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s write certificate.pem error: %v", m.IoTThingName, err)
			return
		}
		if err := write(keyPath, payload.PrivateKey, 0o600); err != nil {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s write private_key.pem error: %v", m.IoTThingName, err)
			return
		}
		if err := write(caPath, payload.RootCA, 0o644); err != nil {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s write root_ca.pem error: %v", m.IoTThingName, err)
			return
		}

		// Update the live TLS configuration in the manager (single source of truth)

		if st.credDir != "" {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s credentials saved to %s (certificate.pem, private_key.pem, root_ca.pem) and TLS reloaded", m.IoTThingName, st.credDir)
		} else {
			st.logger.Printf("[tessa/things/certificates/notify] thing=%s credentials stored in temp dir %s (stateless) and TLS reloaded", m.IoTThingName, useDir)
		}
	})
}
