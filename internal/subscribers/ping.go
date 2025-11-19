package subscribers

import (
	"time"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
)

func registerPing(st *starter) {
	st.On("ping", func(m pubsub.Message) {
		st.logger.Printf("[ping] %s data=%s", m.Time.Format(time.RFC3339), string(m.Data))
	})
}
