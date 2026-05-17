package controller

import (
	"fmt"
	"sync"

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
	TTS        *tts.Registry
	Definition functions.Definition

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
	return &SessionManager{
		TTS:                ttsReg,
		Definition:         def,
		MaxConcurrentCalls: maxCalls,
		sessions:           make(map[string]*Session),
	}
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

	session := NewSession(id, ph, audioSink, rec, m.TTS, m.Definition, callCtl)
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
