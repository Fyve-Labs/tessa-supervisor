package subscribers

import (
	"log"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
	"github.com/Fyve-Labs/tessa-daemon/internal/tunnel"
)

// starter orchestrates subscribing handlers to topics and tracking unsubs.
// Handlers are registered via On within category-specific files.
// Not exported; used internally by Start.
type starter struct {
	ps        pubsub.PubSub
	logger    *log.Logger
	stops     []func()
	credDir   string
	tunnelMgr *tunnel.Manager
}

// On subscribes a handler to a topic and starts a goroutine to consume messages.
func (s *starter) On(topic string, handler func(pubsub.Message)) {
	ch, unsub, err := s.ps.Subscribe(topic)
	if err != nil {
		s.logger.Printf("subscriber for %s failed: %v", topic, err)
		return
	}
	s.stops = append(s.stops, unsub)
	go func() {
		for msg := range ch {
			func() {
				defer func() {
					if r := recover(); r != nil {
						s.logger.Printf("panic in subscriber %s: %v", topic, r)
					}
				}()
				handler(msg)
			}()
		}
	}()
}

// stop returns a function to stop all subscribers created by this starter.
func (s *starter) stop() func() {
	return func() {
		for _, f := range s.stops {
			f()
		}
	}
}

// Start wires all predefined subscribers and returns a stop function.
func Start(ps pubsub.PubSub, logger *log.Logger, credentialsDir string, mgr *tunnel.Manager) func() {
	st := &starter{ps: ps, logger: logger, credDir: credentialsDir, tunnelMgr: mgr}
	registerPing(st)
	registerCredentials(st)
	registerTunnels(st)
	registerCommands(st)
	return st.stop()
}
