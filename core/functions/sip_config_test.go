package functions

import (
	"strings"
	"testing"
)

func preparedSIPDefinition() Definition {
	d := Definition{
		SIP: SIPConfig{
			MaxConcurrentCalls: 10,
			Connections: []SIPConnection{
				{ID: "endpoint", Kind: SIPKindEndpoint, Registration: SIPRegistrationActive, Server: "pbx.test:5060", Extension: "100", Entrypoint: "main"},
				{ID: "trunk", Kind: SIPKindTrunk, Registration: SIPRegistrationInbound, LocalPort: 5061, AllowedCIDRs: []string{"192.0.2.0/24"}, Routes: []SIPRoute{
					{ID: "sales", Number: "+46123", Entrypoint: "sales"},
					{ID: "fallback", CatchAll: true, Entrypoint: "main"},
				}},
			},
		},
		UnsortedFunctions: []Fn{{Name: "main"}, {Name: "sales"}},
		Functions:         make(map[string]*Fn),
	}
	d.Prepare()
	return d
}

func TestSIPRouteResolution(t *testing.T) {
	d := preparedSIPDefinition()
	trunk := d.SIP.Connections[1]
	if got, ok := trunk.ResolveEntrypoint(" +46123 "); !ok || got != "sales" {
		t.Fatalf("exact route = %q, %v; want sales, true", got, ok)
	}
	if got, ok := trunk.ResolveEntrypoint("999"); !ok || got != "main" {
		t.Fatalf("catch-all route = %q, %v; want main, true", got, ok)
	}
	endpoint := d.SIP.Connections[0]
	if got, ok := endpoint.ResolveEntrypoint("anything"); !ok || got != "main" {
		t.Fatalf("endpoint route = %q, %v; want main, true", got, ok)
	}
}

func TestSIPConfigValidation(t *testing.T) {
	d := preparedSIPDefinition()
	if err := d.Validate(); err != nil {
		t.Fatalf("valid definition: %v", err)
	}

	duplicate := d
	duplicate.SIP.Connections = append([]SIPConnection(nil), d.SIP.Connections...)
	duplicate.SIP.Connections[1].Routes = append([]SIPRoute(nil), d.SIP.Connections[1].Routes...)
	duplicate.SIP.Connections[1].Routes[1] = SIPRoute{ID: "duplicate", Number: "46123", Entrypoint: "main"}
	if err := duplicate.Validate(); err == nil || !strings.Contains(err.Error(), "unique after normalization") {
		t.Fatalf("duplicate number error = %v", err)
	}

	untrusted := d
	untrusted.SIP.Connections = append([]SIPConnection(nil), d.SIP.Connections...)
	untrusted.SIP.Connections[1].AllowedCIDRs = nil
	if err := untrusted.Validate(); err == nil || !strings.Contains(err.Error(), "allowed_cidrs") {
		t.Fatalf("missing CIDR error = %v", err)
	}

	unsupported := d
	unsupported.SIP.Connections = append([]SIPConnection(nil), d.SIP.Connections...)
	unsupported.SIP.Connections[0].Transport = "wss"
	if err := unsupported.Validate(); err == nil || !strings.Contains(err.Error(), "transport") {
		t.Fatalf("unsupported transport error = %v", err)
	}
}

func TestNestedTrunkTOMLDecodes(t *testing.T) {
	raw := `
[sip]
max_concurrent_calls = 10
record_path = "files/recording"

[[sip.connection]]
id = "trunk"
name = "Public trunk"
kind = "trunk"
registration = "inbound"
transport = "udp"
local_port = 5062
allowed_cidrs = ["192.0.2.0/24"]

[[sip.connection.route]]
id = "sales"
number = "+46123"
entrypoint = "sales"

[[sip.connection.route]]
id = "fallback"
catch_all = true
entrypoint = "main"

[[fn]]
name = "main"

[[fn]]
name = "sales"
`
	d, err := Decode([]byte(raw))
	if err != nil {
		t.Fatalf("decode nested SIP TOML: %v", err)
	}
	if len(d.SIP.Connections) != 1 || len(d.SIP.Connections[0].Routes) != 2 {
		t.Fatalf("decoded connections/routes = %d/%d, want 1/2", len(d.SIP.Connections), len(d.SIP.Connections[0].Routes))
	}
}
