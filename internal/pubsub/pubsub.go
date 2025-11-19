package pubsub

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Message is the unit published to topics and received by subscribers.
type Message struct {
	Topic        string          `json:"topic"`
	Data         json.RawMessage `json:"data"`
	Time         time.Time       `json:"time"`
	IoTThingName string          `json:"iot_thing_name"`
}

// PubSub defines a minimal publish/subscribe interface.
type PubSub interface {
	Publish(topic string, msg Message) error
	Subscribe(topic string) (<-chan Message, func(), error)
}

// inMemoryPubSub is a simple in-memory implementation of PubSub.
type inMemoryPubSub struct {
	mu      sync.RWMutex
	subs    map[string]map[chan Message]struct{}
	closed  bool
	closeCh chan struct{}
}

// NewInMemory creates a new in-memory pub/sub instance.
func NewInMemory() *inMemoryPubSub {
	return &inMemoryPubSub{
		subs:    make(map[string]map[chan Message]struct{}),
		closeCh: make(chan struct{}),
	}
}

func (ps *inMemoryPubSub) Publish(topic string, msg Message) error {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if ps.closed {
		return errors.New("pubsub closed")
	}
	m := ps.subs[topic]
	for ch := range m {
		// non-blocking send; drop if subscriber is slow
		select {
		case ch <- msg:
		default:
		}
	}
	return nil
}

func (ps *inMemoryPubSub) Subscribe(topic string) (<-chan Message, func(), error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return nil, nil, errors.New("pubsub closed")
	}
	ch := make(chan Message, 64)
	if ps.subs[topic] == nil {
		ps.subs[topic] = make(map[chan Message]struct{})
	}
	ps.subs[topic][ch] = struct{}{}
	unsub := func() {
		ps.mu.Lock()
		defer ps.mu.Unlock()
		if m := ps.subs[topic]; m != nil {
			delete(m, ch)
			close(ch)
			if len(m) == 0 {
				delete(ps.subs, topic)
			}
		}
	}
	return ch, unsub, nil
}

// Close shuts down the pubsub and closes all subscriber channels.
func (ps *inMemoryPubSub) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if ps.closed {
		return
	}
	ps.closed = true
	for topic, m := range ps.subs {
		for ch := range m {
			close(ch)
		}
		delete(ps.subs, topic)
	}
	close(ps.closeCh)
}
