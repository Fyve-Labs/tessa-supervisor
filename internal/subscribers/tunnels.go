package subscribers

import (
	"encoding/json"
	"fmt"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
)

// registerTunnels wires subscribers for tunnel notification topics.
func registerTunnels(st *starter) {
	st.On("$aws/things/tunnels/notify", func(m pubsub.Message) {
		if m.IoTThingName != "" {
			st.logger.Printf("[$aws/things/tunnels/notify] thing=%s AWS IoT tunnel notification: %s", m.IoTThingName, string(m.Data))
		} else {
			st.logger.Printf("[$aws/things/tunnels/notify] AWS IoT tunnel notification: %s", string(m.Data))
		}
	})
	st.On("tessa/things/tunnels/notify", func(m pubsub.Message) {
		// Expected payload structure:
		// {
		//   "server": "tunnel server address: https://192.168.1.9:8080",
		//   "tunnel_port": "Free port that server assigned to this device for SSH tunneling: example 2222",
		// }
		var payload struct {
			Server     string `json:"server"`
			ServerPort int    `json:"server_port"`
			ClientPort int    `json:"client_port"`
		}
		if err := json.Unmarshal(m.Data, &payload); err != nil {
			st.logger.Printf("[tessa/things/tunnels/notify] thing=%s invalid payload JSON: %v", m.IoTThingName, err)
			return
		}
		if payload.Server == "" || payload.ServerPort == 0 || payload.ClientPort == 0 {
			st.logger.Printf("[tessa/things/tunnels/notify] thing=%s missing required fields (server/server_port/client_port)", m.IoTThingName)
			return
		}

		sshRemote := fmt.Sprintf("R:%d:localhost:%d", payload.ServerPort, payload.ClientPort)
		st.tunnelMgr.SetServer(payload.Server)
		st.tunnelMgr.AddRemote(sshRemote)

		if err := st.tunnelMgr.Restart(); err != nil {
			st.logger.Printf("[tessa/things/tunnels/notify] thing=%s start tunnel failed: %v", m.IoTThingName, err)
			return
		}

		// TLS config (if any) is managed at startup and by the credentials subscriber; do not override here.
		st.logger.Printf("[tessa/things/tunnels/notify] thing=%s tunnel configuration applied: server=%s remotes=%v", m.IoTThingName, payload.Server, sshRemote)
	})
}
