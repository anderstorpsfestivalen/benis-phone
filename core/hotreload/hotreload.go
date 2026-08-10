// Package hotreload owns the runtime config reload behaviour for the IVR
// binary: it subscribes to the worker's ConfigBroker over WebSocket,
// optionally polls the signed /bridge/hash endpoint as a fallback, listens for SIGUSR1,
// and on every trigger re-syncs the R2 file mirror before reconciling the
// active flow definition, SIP listeners, registrations, and routes.
//
// One Manager per running binary. Start spawns goroutines for each
// transport; Stop tears them all down for graceful shutdown. The Manager
// is safe to use concurrently — reloadOnce serializes through hashMu.
package hotreload

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/sip"
)

// Config is the set of dependencies and tunables a Manager needs.
type Config struct {
	// RemoteClient signs requests to /bridge/hash, /bridge/runtime, and /bridge/ws.
	RemoteClient *functions.RemoteClient
	// SIPSupervisor reconciles both flow snapshots and connection lifecycles.
	SIPSupervisor *sip.Supervisor
	// ApplyRuntime atomically prepares and installs every credential-dependent
	// resource before reconciling SIP. When nil, the legacy supervisor-only
	// apply is used by focused tests.
	ApplyRuntime func(functions.RuntimeConfig) error
	InstanceID   string
	Status       <-chan []byte
	// StatusSnapshot is replayed after each WebSocket reconnect.
	StatusSnapshot func() [][]byte
	// InitialHash is the hash observed at startup. Used to short-circuit
	// the first WS push if it matches.
	InitialHash string
	// Poll enables the legacy HTTP poll fallback alongside the WS
	// subscription. Useful when the WS upgrade is blocked.
	Poll bool
	// PollInterval is the tick between hash checks when Poll is true.
	// Zero disables polling regardless of Poll.
	PollInterval time.Duration

	Logger *logrus.Logger
}

// Manager drives config reloads for one running binary.
type Manager struct {
	cfg Config
	log *logrus.Logger

	hashMu sync.Mutex
	hash   string

	wsCtx    context.Context
	wsCancel context.CancelFunc
	stopPoll chan struct{}
	usr1     chan os.Signal
	wg       sync.WaitGroup
	started  bool
}

// New constructs a Manager. Call Start to spawn its goroutines.
func New(cfg Config) *Manager {
	return &Manager{
		cfg:  cfg,
		log:  cfg.Logger,
		hash: cfg.InitialHash,
	}
}

// Start kicks off the WS subscription, the optional poll loop, and the
// SIGUSR1 handler. Safe to call exactly once per Manager.
func (m *Manager) Start() {
	if m.started {
		return
	}
	m.started = true

	m.wsCtx, m.wsCancel = context.WithCancel(context.Background())
	m.stopPoll = make(chan struct{})

	// WS subscription — the primary reload trigger.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		w := &functions.WSWatcher{
			RemoteClient: m.cfg.RemoteClient,
			InstanceID:   m.cfg.InstanceID,
			Outgoing:     m.cfg.Status,
			Snapshot:     m.cfg.StatusSnapshot,
			Logger:       m.log,
			OnUpdate: func(_ string) {
				m.reloadOnce(false)
			},
		}
		w.Run(m.wsCtx)
	}()

	// Optional poll fallback.
	if m.cfg.Poll && m.cfg.PollInterval > 0 {
		m.wg.Add(1)
		go m.runPoll()
	}

	// SIGUSR1 — manual force-reload trigger for ops.
	m.usr1 = make(chan os.Signal, 1)
	signal.Notify(m.usr1, syscall.SIGUSR1)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for range m.usr1 {
			m.log.Info("SIGUSR1 received, forcing config reload")
			m.reloadOnce(true)
		}
	}()
}

// Stop signals every goroutine and waits for them to exit.
func (m *Manager) Stop() {
	if !m.started {
		return
	}
	close(m.stopPoll)
	m.wsCancel()
	signal.Stop(m.usr1)
	close(m.usr1)
	m.wg.Wait()
}

// Force triggers a reload immediately, bypassing the hash equality check.
// Mainly useful from tests.
func (m *Manager) Force() { m.reloadOnce(true) }

func (m *Manager) runPoll() {
	defer m.wg.Done()
	t := time.NewTicker(m.cfg.PollInterval)
	defer t.Stop()
	m.log.WithField("interval", m.cfg.PollInterval.String()).Info("Hot-reload poll loop started")
	for {
		select {
		case <-m.stopPoll:
			m.log.Info("Hot-reload poll loop stopped")
			return
		case <-t.C:
			m.reloadOnce(false)
		}
	}
}

// reloadOnce pulls the worker's current hash; if it differs from the
// last one we saw (or force is set), it re-syncs the R2 bucket so any
// newly-referenced audio files land on disk before the new runtime bundle
// is applied. The hash mutex serialises concurrent triggers
// (WS event + poll tick + SIGUSR1) so we never run two reloads in
// parallel and never observe a torn currentHash.
func (m *Manager) reloadOnce(force bool) {
	m.hashMu.Lock()
	defer m.hashMu.Unlock()

	h, err := m.cfg.RemoteClient.FetchHash()
	if err != nil {
		m.log.Warnf("hot-reload: hash fetch failed: %v", err)
		return
	}
	if !force && h == m.hash {
		return
	}
	runtime, err := m.cfg.RemoteClient.LoadRuntimeConfig()
	if err != nil {
		m.log.Warnf("hot-reload: definition fetch failed: %v", err)
		return
	}
	if m.cfg.ApplyRuntime != nil {
		err = m.cfg.ApplyRuntime(runtime)
	} else if m.cfg.SIPSupervisor != nil {
		err = m.cfg.SIPSupervisor.Apply(runtime.Definition, runtime.SIPPasswords)
	} else {
		err = fmt.Errorf("no runtime applier configured")
	}
	if err != nil {
		m.log.Warnf("hot-reload: apply failed: %v", err)
		return
	}
	appliedHash := runtime.Revision
	if appliedHash == "" {
		appliedHash = h
	}
	m.hash = appliedHash
	m.log.WithField("hash", ShortHash(appliedHash)).Info("Hot-reloaded config")
}

// ShortHash returns the first 8 characters of a hex digest for tidy log
// output. Exported because the main package logs the initial hash before
// the Manager exists.
func ShortHash(h string) string {
	if len(h) < 8 {
		return h
	}
	return h[:8]
}
