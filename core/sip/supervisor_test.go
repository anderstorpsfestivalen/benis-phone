package sip

import (
	"testing"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
)

func TestAllocatePortsUsesDedicatedStablePorts(t *testing.T) {
	in := []functions.SIPConnection{
		{ID: "automatic-a", LocalPort: 0},
		{ID: "explicit", LocalPort: 5060},
		{ID: "automatic-b", LocalPort: 0},
	}
	out := allocatePorts(in)
	if out[0].LocalPort != 5061 || out[1].LocalPort != 5060 || out[2].LocalPort != 5062 {
		t.Fatalf("allocated ports = [%d %d %d], want [5061 5060 5062]", out[0].LocalPort, out[1].LocalPort, out[2].LocalPort)
	}
	if in[0].LocalPort != 0 {
		t.Fatal("allocatePorts mutated its input")
	}
}

func TestAllocatePortsRemainStableAcrossInsertAndReorder(t *testing.T) {
	s := &Supervisor{autoPorts: make(map[string]int)}
	first := s.allocatePortsLocked([]functions.SIPConnection{
		{ID: "bravo", Transport: "udp"},
		{ID: "charlie", Transport: "udp"},
	})
	if first[0].LocalPort != 5060 || first[1].LocalPort != 5061 {
		t.Fatalf("first allocation = [%d %d], want [5060 5061]", first[0].LocalPort, first[1].LocalPort)
	}

	second := s.allocatePortsLocked([]functions.SIPConnection{
		{ID: "charlie", Transport: "udp"},
		{ID: "alpha", Transport: "udp"},
		{ID: "bravo", Transport: "udp"},
	})
	got := map[string]int{}
	for _, conn := range second {
		got[conn.ID] = conn.LocalPort
	}
	if got["bravo"] != 5060 || got["charlie"] != 5061 || got["alpha"] != 5062 {
		t.Fatalf("stable allocation = %#v, want bravo=5060 charlie=5061 alpha=5062", got)
	}
}

func TestAllocatePortsIsPerTransport(t *testing.T) {
	s := &Supervisor{autoPorts: make(map[string]int)}
	out := s.allocatePortsLocked([]functions.SIPConnection{
		{ID: "udp", Transport: "UDP"},
		{ID: "tcp", Transport: "tcp"},
	})
	if out[0].LocalPort != 5060 || out[1].LocalPort != 5060 {
		t.Fatalf("per-transport ports = [%d %d], want [5060 5060]", out[0].LocalPort, out[1].LocalPort)
	}
}

func TestListenerChangeClassification(t *testing.T) {
	base := normalizeClientConfig(ClientConfig{
		ConnectionID: "primary", Registration: functions.SIPRegistrationActive,
		Server: "pbx.test:5060", Extension: "100", Transport: "UDP", LocalPort: 5060,
	})

	routeOnly := base
	routeOnly.Entrypoint = "sales"
	if listenerRequiresRecreate(base, routeOnly) {
		t.Fatal("route-only change should update in place")
	}

	passwordOnly := base
	passwordOnly.Password = "new"
	if listenerRequiresRecreate(base, passwordOnly) || !registrationChanged(base, passwordOnly) {
		t.Fatal("password change should restart registration without recreating listener")
	}

	for name, mutate := range map[string]func(*ClientConfig){
		"server":       func(cfg *ClientConfig) { cfg.Server = "other.test:5060" },
		"identity":     func(cfg *ClientConfig) { cfg.Extension = "101" },
		"domain":       func(cfg *ClientConfig) { cfg.Domain = "other.test" },
		"transport":    func(cfg *ClientConfig) { cfg.Transport = "tcp" },
		"port":         func(cfg *ClientConfig) { cfg.LocalPort = 5061 },
		"external ip":  func(cfg *ClientConfig) { cfg.ExternalIP = "192.0.2.1" },
		"inbound mode": func(cfg *ClientConfig) { cfg.InboundOnly = true },
	} {
		t.Run(name, func(t *testing.T) {
			changed := base
			mutate(&changed)
			if !listenerRequiresRecreate(base, changed) {
				t.Fatalf("%s change should recreate listener", name)
			}
		})
	}
}

func TestInboundTransportListensOnAllIPv4Interfaces(t *testing.T) {
	transport, err := buildTransport(ClientConfig{
		Transport:   "udp",
		LocalPort:   5010,
		InboundOnly: true,
	}, "45.154.28.21")
	if err != nil {
		t.Fatal(err)
	}
	if transport.BindHost != "0.0.0.0" || transport.BindPort != 5010 {
		t.Fatalf("inbound bind = %s:%d, want 0.0.0.0:5010", transport.BindHost, transport.BindPort)
	}
	if transport.ExternalHost != "45.154.28.21" || !transport.MediaExternalIP.Equal([]byte{45, 154, 28, 21}) {
		t.Fatalf("inbound advertised address = %q/%v, want detected IP", transport.ExternalHost, transport.MediaExternalIP)
	}

	transport, err = buildTransport(ClientConfig{
		Transport:   "udp",
		LocalPort:   5010,
		InboundOnly: true,
		ExternalIP:  "203.0.113.7",
	}, "10.0.0.7")
	if err != nil {
		t.Fatal(err)
	}
	if transport.BindHost != "0.0.0.0" || transport.ExternalHost != "203.0.113.7" || !transport.MediaExternalIP.Equal([]byte{203, 0, 113, 7}) {
		t.Fatalf("NAT transport = bind %q, external %q/%v", transport.BindHost, transport.ExternalHost, transport.MediaExternalIP)
	}

	registered, err := buildTransport(ClientConfig{
		Transport: "udp",
		LocalPort: 5060,
	}, "10.0.0.7")
	if err != nil {
		t.Fatal(err)
	}
	if registered.BindHost != "10.0.0.7" || registered.ExternalHost != "" || registered.MediaExternalIP != nil {
		t.Fatalf("registered transport unexpectedly changed: %#v", registered)
	}
}

func TestStatusSnapshotKeepsLatestEvent(t *testing.T) {
	s := &Supervisor{current: make(map[string]StatusEvent)}
	s.report(newStatus("b", "starting", "", "", 5061))
	s.report(newStatus("a", "ready", "", "", 5060))
	s.report(newStatus("b", "error", "registration_failed", "failed", 5061))

	snapshot := s.StatusSnapshot()
	if len(snapshot) != 2 || snapshot[0].ConnectionID != "a" || snapshot[1].ConnectionID != "b" {
		t.Fatalf("snapshot order/content = %#v", snapshot)
	}
	if snapshot[1].State != "error" || snapshot[1].Code != "registration_failed" {
		t.Fatalf("latest b status = %#v", snapshot[1])
	}
}

func TestIncomingCallReservationCountsAsActive(t *testing.T) {
	def := preparedClientDefinition("main")
	client := &Client{
		config:     ClientConfig{ConnectionID: "primary", Entrypoint: "main"},
		definition: def,
		accepting:  true,
	}
	cfg, snapshot, ok := client.beginIncomingCall()
	if !ok || cfg.ConnectionID != "primary" || snapshot.Functions["main"] == nil {
		t.Fatalf("incoming snapshot = %#v, functions=%v, ok=%v", cfg, snapshot.Functions, ok)
	}
	if got := client.ActiveCalls(); got != 1 {
		t.Fatalf("active calls with pending setup = %d, want 1", got)
	}
	client.releasePendingCall()
	if got := client.ActiveCalls(); got != 0 {
		t.Fatalf("active calls after release = %d, want 0", got)
	}
}

func preparedClientDefinition(names ...string) functions.Definition {
	def := functions.Definition{
		UnsortedFunctions: make([]functions.Fn, 0, len(names)),
		Functions:         make(map[string]*functions.Fn),
	}
	for _, name := range names {
		def.UnsortedFunctions = append(def.UnsortedFunctions, functions.Fn{Name: name})
	}
	def.Prepare()
	return def
}

func TestSourceAllowed(t *testing.T) {
	if !sourceAllowed("192.0.2.10:5060", []string{"192.0.2.0/24"}) {
		t.Fatal("expected source inside CIDR to be accepted")
	}
	if sourceAllowed("198.51.100.10:5060", []string{"192.0.2.0/24"}) {
		t.Fatal("expected source outside CIDR to be rejected")
	}
	if !sourceAllowed("198.51.100.10:5060", nil) {
		t.Fatal("empty optional allowlist should accept registered provider traffic")
	}
}
