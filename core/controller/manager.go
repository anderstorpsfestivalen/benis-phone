package controller

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/callctl"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
	log "github.com/sirupsen/logrus"
)

// SessionManager manages multiple concurrent call sessions.
type SessionManager struct {
	// Shared resources
	TTS *tts.Registry

	// definition is swappable at runtime (see UpdateDefinition). Sessions
	// snapshot it at construction time, so in-flight calls keep the
	// config they started with — only new calls pick up the swap.
	definition atomic.Pointer[functions.Definition]

	// Configuration
	MaxConcurrentCalls int

	// Active sessions
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager with shared resources.
func NewSessionManager(ttsReg *tts.Registry, def functions.Definition, maxCalls int) *SessionManager {
	if maxCalls <= 0 {
		maxCalls = 10 // default
	}
	m := &SessionManager{
		TTS:                ttsReg,
		MaxConcurrentCalls: maxCalls,
		sessions:           make(map[string]*Session),
	}
	m.definition.Store(&def)
	return m
}

// Definition returns the current config snapshot. The returned value is
// safe to read concurrently and to copy by value; do not mutate it.
func (m *SessionManager) Definition() functions.Definition {
	return *m.definition.Load()
}

// UpdateDefinition atomically replaces the active config. In-flight
// sessions are unaffected (they hold their own snapshot via NewSession);
// every session created after this call sees the new definition.
func (m *SessionManager) UpdateDefinition(def functions.Definition) {
	m.definition.Store(&def)
}

// CreateSession creates and registers a new session with the given components.
// callCtl may be nil for backends that don't support call control. Returns an
// error if the maximum concurrent calls limit is reached.
func (m *SessionManager) CreateSession(id string, ph phone.FlowPhone, audioSink audio.AudioSink, rec audio.AudioSource, callCtl callctl.Controller) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.MaxConcurrentCalls {
		return nil, fmt.Errorf("maximum concurrent calls (%d) reached", m.MaxConcurrentCalls)
	}

	if _, exists := m.sessions[id]; exists {
		return nil, fmt.Errorf("session %s already exists", id)
	}

	session := NewSession(id, ph, audioSink, rec, m.TTS, m.Definition(), callCtl)
	m.sessions[id] = session

	log.WithFields(log.Fields{
		"session_id":     id,
		"active_calls":   len(m.sessions),
		"max_calls":      m.MaxConcurrentCalls,
	}).Info("Session created")

	return session, nil
}

// GetSession returns a session by ID, or nil if not found.
func (m *SessionManager) GetSession(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

// RemoveSession removes and stops a session by ID.
func (m *SessionManager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists := m.sessions[id]; exists {
		session.Stop()
		delete(m.sessions, id)
		log.WithFields(log.Fields{
			"session_id":   id,
			"active_calls": len(m.sessions),
		}).Info("Session removed")
	}
}

// ActiveSessionCount returns the number of active sessions.
func (m *SessionManager) ActiveSessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// StopAll stops all active sessions.
func (m *SessionManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		session.Stop()
		delete(m.sessions, id)
	}
	log.Info("All sessions stopped")
}
