package ssh

import (
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type activeSession struct {
	conn    ssh.Conn
	channel ssh.Channel
}

// sessionManager safely manages active SSH sessions.
type sessionManager struct {
	sessions map[string]*activeSession
	mu       sync.Mutex
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		sessions: make(map[string]*activeSession),
	}
}

// Add an active session to the manager.
func (sm *sessionManager) Add(conn ssh.Conn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[conn.RemoteAddr().String()] = &activeSession{conn: conn}
}

// SetChannel associates a PTY channel with a connection.
func (sm *sessionManager) SetChannel(conn ssh.Conn, channel ssh.Channel) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sess, ok := sm.sessions[conn.RemoteAddr().String()]; ok {
		sess.channel = channel
	}
}

// Remove a session from the manager.
func (sm *sessionManager) Remove(conn ssh.Conn) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, conn.RemoteAddr().String())
}

// Broadcast a message to all active PTY sessions and then close them.
func (sm *sessionManager) BroadcastAndClose(message string, gracePeriod time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Announce to all sessions with an active channel
	for _, s := range sm.sessions {
		if s.channel != nil {
			fmt.Fprintf(s.channel.Stderr(), "\r\n\n%s\r\n", message)
		}
	}

	// Wait for the grace period if there are any active sessions
	if len(sm.sessions) > 0 && gracePeriod > 0 {
		log.Printf("Waiting for grace period: %s", gracePeriod)
		time.Sleep(gracePeriod)
	}

	// Close all connections
	log.Println("Closing all active connections...")
	for addr, s := range sm.sessions {
		if err := s.conn.Close(); err != nil {
			log.Printf("Failed to close connection for %s: %v", addr, err)
		}
	}
}
