package sip

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/controller"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
	log "github.com/sirupsen/logrus"
)

// Supervisor reconciles independently configured SIP listeners while sharing
// one SessionManager (and therefore one global concurrent-call limit). A
// listener owns its routing and Definition snapshot so a failed replacement
// can continue serving its last known-good configuration.
type Supervisor struct {
	mu         sync.Mutex
	clients    map[string]*Client
	draining   map[*Client]struct{}
	desired    map[string]desiredClient
	generation map[string]uint64
	autoPorts  map[string]int
	drainWG    sync.WaitGroup
	manager    *controller.SessionManager
	status     func(StatusEvent)
	statusMu   sync.RWMutex
	current    map[string]StatusEvent
	log        *log.Logger
	stopped    bool
}

type desiredClient struct {
	config     ClientConfig
	definition functions.Definition
}

func NewSupervisor(ttsReg *tts.Registry, def functions.Definition, status func(StatusEvent), logger *log.Logger) *Supervisor {
	max := def.SIP.MaxConcurrentCalls
	if max <= 0 {
		max = 10
	}
	return &Supervisor{
		clients:    make(map[string]*Client),
		draining:   make(map[*Client]struct{}),
		desired:    make(map[string]desiredClient),
		generation: make(map[string]uint64),
		autoPorts:  make(map[string]int),
		manager:    controller.NewSessionManager(ttsReg, def, max),
		status:     status,
		current:    make(map[string]StatusEvent),
		log:        logger,
	}
}

func (s *Supervisor) SessionManager() *controller.SessionManager { return s.manager }

func (s *Supervisor) Start(def functions.Definition, passwords map[string]string) error {
	return s.Apply(def, passwords)
}

// Apply reconciles each listener independently. Listener setup failures leave
// the old listener and its flow snapshot intact. Changes that need the same
// socket are drained and retried automatically; a later Apply supersedes any
// pending replacement through the per-connection generation counter.
func (s *Supervisor) Apply(def functions.Definition, passwords map[string]string) error {
	if err := def.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return fmt.Errorf("SIP supervisor is stopped")
	}

	connections := s.allocatePortsLocked(def.SIP.Connections)
	s.manager.UpdateDefinition(def) // retained for diagnostics and compatibility
	s.manager.UpdateMaxConcurrentCalls(def.SIP.MaxConcurrentCalls)

	nextDesired := make(map[string]desiredClient, len(connections))
	missingCredentials := make(map[string]bool)
	ids := make([]string, 0, len(connections))
	for _, conn := range connections {
		cfg := clientConfigFromDefinition(def.SIP, conn, passwords[conn.ID])
		ids = append(ids, conn.ID)
		s.generation[conn.ID]++
		if cfg.Password == "" {
			missingCredentials[conn.ID] = true
			s.report(newStatus(conn.ID, "error", "missing_credentials", "SIP password has not been configured", cfg.LocalPort))
			continue
		}
		nextDesired[conn.ID] = desiredClient{config: cfg, definition: def}
	}
	sort.Strings(ids)
	s.desired = nextDesired

	// Snapshot socket ownership before making changes. It lets us stage a
	// different-port replacement without interrupting its old listener while
	// deferring swaps and same-port replacements until their owners drain.
	owners := make(map[string]*Client, len(s.clients))
	for _, client := range s.clients {
		owners[socketKey(client.Config())] = client
	}
	deferred := make(map[string]bool)

	for _, id := range ids {
		want, ok := s.desired[id]
		if !ok {
			continue
		}
		old := s.clients[id]
		if old == nil {
			if owners[socketKey(want.config)] != nil || s.drainingOwnsSocketLocked(want.config) {
				deferred[id] = true
				continue
			}
			if !s.startDesiredLocked(id, s.generation[id]) {
				s.scheduleDesiredLocked(id, s.generation[id])
			}
			if client := s.clients[id]; client != nil {
				owners[socketKey(client.Config())] = client
			}
			continue
		}

		oldCfg := old.Config()
		if !listenerRequiresRecreate(oldCfg, want.config) {
			old.Update(want.config, want.definition, registrationChanged(oldCfg, want.config))
			continue
		}

		owner := owners[socketKey(want.config)]
		canStage := socketKey(oldCfg) != socketKey(want.config) && owner == nil && !s.drainingOwnsSocketLocked(want.config)
		if canStage {
			replacement, err := s.newStartedClient(want.config, want.definition)
			if err != nil {
				s.report(newStatus(id, "error", "start_failed", err.Error(), want.config.LocalPort))
				continue
			}
			old.silenceStatus()
			s.clients[id] = replacement
			delete(owners, socketKey(oldCfg))
			owners[socketKey(want.config)] = replacement
			s.beginDrainLocked(old)
			continue
		}
		deferred[id] = true
	}

	// Drain removed, credential-less, and deferred listeners as one set before
	// starting anything that may need one of their sockets. This handles port
	// swaps and connection-ID changes without requiring a second config save.
	for id, client := range s.clients {
		_, wanted := s.desired[id]
		if wanted && !deferred[id] {
			continue
		}
		if !wanted && !missingCredentials[id] {
			s.generation[id]++
		}
		delete(s.clients, id)
		delete(owners, socketKey(client.Config()))
		s.beginDrainLocked(client)
	}

	for id := range deferred {
		s.scheduleDesiredLocked(id, s.generation[id])
	}
	return nil
}

// allocatePorts remains as a deterministic one-shot helper for tests and
// callers that do not need reload stability.
func allocatePorts(in []functions.SIPConnection) []functions.SIPConnection {
	s := Supervisor{autoPorts: make(map[string]int)}
	return s.allocatePortsLocked(in)
}

// allocatePortsLocked keeps an automatically allocated port attached to its
// stable connection ID. New automatic connections receive the first free port
// for their transport without shifting existing connections.
func (s *Supervisor) allocatePortsLocked(in []functions.SIPConnection) []functions.SIPConnection {
	out := append([]functions.SIPConnection(nil), in...)
	used := make(map[string]bool)
	presentAuto := make(map[string]bool)
	for _, conn := range out {
		if conn.LocalPort > 0 {
			used[connectionSocketKey(conn.Transport, conn.LocalPort)] = true
			delete(s.autoPorts, conn.ID)
		}
	}
	automatic := make([]int, 0)
	for i := range out {
		if out[i].LocalPort != 0 {
			continue
		}
		presentAuto[out[i].ID] = true
		if port := s.autoPorts[out[i].ID]; port > 0 && !used[connectionSocketKey(out[i].Transport, port)] {
			out[i].LocalPort = port
			used[connectionSocketKey(out[i].Transport, port)] = true
			continue
		}
		automatic = append(automatic, i)
	}
	sort.Slice(automatic, func(i, j int) bool { return out[automatic[i]].ID < out[automatic[j]].ID })
	for _, i := range automatic {
		port := 5060
		for used[connectionSocketKey(out[i].Transport, port)] {
			port++
		}
		out[i].LocalPort = port
		s.autoPorts[out[i].ID] = port
		used[connectionSocketKey(out[i].Transport, port)] = true
	}
	for id := range s.autoPorts {
		if !presentAuto[id] {
			delete(s.autoPorts, id)
		}
	}
	return out
}

func connectionSocketKey(transport string, port int) string {
	transport = strings.ToLower(strings.TrimSpace(transport))
	if transport == "" {
		transport = "udp"
	}
	return fmt.Sprintf("%s:%d", transport, port)
}

func socketKey(cfg ClientConfig) string {
	return connectionSocketKey(cfg.Transport, cfg.LocalPort)
}

func clientConfigFromDefinition(global functions.SIPConfig, conn functions.SIPConnection, password string) ClientConfig {
	transport := conn.Transport
	if transport == "" {
		transport = "udp"
	}
	expiry := conn.ExpirySeconds
	if expiry <= 0 {
		expiry = 300
	}
	recordPath := global.RecordPath
	if recordPath == "" {
		recordPath = "files/recording"
	}
	return normalizeClientConfig(ClientConfig{
		ConnectionID: conn.ID, Name: conn.Name, Kind: conn.Kind, Registration: conn.Registration,
		Entrypoint: conn.Entrypoint, Routes: append([]functions.SIPRoute(nil), conn.Routes...),
		AllowedCIDRs: append([]string(nil), conn.AllowedCIDRs...), Server: conn.Server,
		Extension: conn.Extension, Username: conn.Username, Password: password, Domain: conn.Domain,
		Transport: transport, LocalPort: conn.LocalPort, ExpirySeconds: expiry,
		RecordPath: recordPath, ExternalIP: conn.ExternalIP,
		InboundOnly: conn.Registration == functions.SIPRegistrationInbound,
	})
}

func listenerRequiresRecreate(a, b ClientConfig) bool {
	return a.Transport != b.Transport || a.LocalPort != b.LocalPort ||
		a.ExternalIP != b.ExternalIP || a.InboundOnly != b.InboundOnly ||
		a.Registration != b.Registration || a.Server != b.Server ||
		a.Extension != b.Extension || a.Domain != b.Domain
}

func registrationChanged(a, b ClientConfig) bool {
	return a.Username != b.Username || a.Password != b.Password || a.ExpirySeconds != b.ExpirySeconds
}

func (s *Supervisor) newStartedClient(cfg ClientConfig, def functions.Definition) (*Client, error) {
	client, err := NewClient(cfg, def, s.manager, s.report)
	if err != nil {
		return nil, err
	}
	if err := client.Start(); err != nil {
		client.Stop()
		return nil, err
	}
	return client, nil
}

// startDesiredLocked returns false only for an operational startup failure
// that is safe to retry while this generation remains desired.
func (s *Supervisor) startDesiredLocked(id string, generation uint64) bool {
	if s.stopped || s.generation[id] != generation || s.clients[id] != nil {
		return true
	}
	want, ok := s.desired[id]
	if !ok {
		return true
	}
	// A rapid follow-up save may move the desired connection to a free socket
	// while its previous generation is still draining elsewhere. Prevent that
	// old generation's eventual "stopped" event from overwriting the new state.
	s.silenceDrainingConnectionLocked(id)
	client, err := s.newStartedClient(want.config, want.definition)
	if err != nil {
		s.report(newStatus(id, "error", "start_failed", err.Error(), want.config.LocalPort))
		return false
	}
	s.clients[id] = client
	return true
}

func (s *Supervisor) silenceDrainingConnectionLocked(id string) {
	for client := range s.draining {
		if client.Config().ConnectionID == id {
			client.silenceStatus()
		}
	}
}

func (s *Supervisor) scheduleDesiredLocked(id string, generation uint64) {
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			s.mu.Lock()
			if s.stopped || s.generation[id] != generation || s.clients[id] != nil {
				s.mu.Unlock()
				return
			}
			want, ok := s.desired[id]
			if !ok {
				s.mu.Unlock()
				return
			}
			if !s.socketBusyLocked(want.config) {
				started := s.startDesiredLocked(id, generation)
				s.mu.Unlock()
				if started {
					return
				}
				time.Sleep(time.Second)
				continue
			}
			s.mu.Unlock()
			<-ticker.C
		}
	}()
}

func (s *Supervisor) socketBusyLocked(cfg ClientConfig) bool {
	want := socketKey(cfg)
	for _, client := range s.clients {
		if socketKey(client.Config()) == want {
			return true
		}
	}
	return s.drainingOwnsSocketLocked(cfg)
}

func (s *Supervisor) drainingOwnsSocketLocked(cfg ClientConfig) bool {
	want := socketKey(cfg)
	for client := range s.draining {
		if socketKey(client.Config()) == want {
			return true
		}
	}
	return false
}

func (s *Supervisor) beginDrainLocked(client *Client) {
	if _, exists := s.draining[client]; exists {
		return
	}
	s.draining[client] = struct{}{}
	client.Drain()
	s.drainWG.Add(1)
	go func() {
		defer s.drainWG.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for client.ActiveCalls() > 0 {
			<-ticker.C
		}
		client.Stop()
		s.mu.Lock()
		delete(s.draining, client)
		s.mu.Unlock()
	}()
}

func (s *Supervisor) report(event StatusEvent) {
	s.statusMu.Lock()
	s.current[event.ConnectionID] = event
	s.statusMu.Unlock()
	if s.status != nil {
		s.status(event)
	}
	if s.log != nil {
		s.log.WithFields(log.Fields{"connection": event.ConnectionID, "state": event.State, "code": event.Code}).Info(event.Message)
	}
}

// StatusSnapshot returns the latest known state for every connection. It is
// replayed whenever the control-plane WebSocket reconnects.
func (s *Supervisor) StatusSnapshot() []StatusEvent {
	s.statusMu.RLock()
	out := make([]StatusEvent, 0, len(s.current))
	for _, event := range s.current {
		out = append(out, event)
	}
	s.statusMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ConnectionID < out[j].ConnectionID })
	return out
}

func (s *Supervisor) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	all := make(map[*Client]struct{}, len(s.clients)+len(s.draining))
	for _, client := range s.clients {
		all[client] = struct{}{}
	}
	for client := range s.draining {
		all[client] = struct{}{}
	}
	s.clients = make(map[string]*Client)
	s.mu.Unlock()

	clients := make([]*Client, 0, len(all))
	for client := range all {
		clients = append(clients, client)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].Config().ConnectionID < clients[j].Config().ConnectionID })
	for _, client := range clients {
		client.Stop()
	}
	s.drainWG.Wait()
	s.manager.StopAll()
}
