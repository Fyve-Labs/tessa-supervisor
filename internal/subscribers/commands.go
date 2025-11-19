package subscribers

import (
	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
)

// registerCommands wires subscribers for device command notifications.
func registerCommands(st *starter) {
	st.On("tessa/things/commands/notify", func(m pubsub.Message) {
		if m.IoTThingName != "" {
			st.logger.Printf("[tessa/things/commands/notify] thing=%s Incoming device command notification: %s", m.IoTThingName, string(m.Data))
		} else {
			st.logger.Printf("[tessa/things/commands/notify] Incoming device command notification: %s", string(m.Data))
		}
	})
}
