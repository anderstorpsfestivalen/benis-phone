package functions

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var sipConfigIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

const (
	SIPKindEndpoint        = "endpoint"
	SIPKindTrunk           = "trunk"
	SIPRegistrationActive  = "registered"
	SIPRegistrationInbound = "inbound"
)

// NormalizeCalledNumber implements the deliberately small v1 routing rule:
// trim whitespace and ignore one leading international '+' marker.
func NormalizeCalledNumber(number string) string {
	number = strings.TrimSpace(number)
	return strings.TrimPrefix(number, "+")
}

// ResolveEntrypoint maps an accepted INVITE on this connection to the
// function where its session starts.
func (c SIPConnection) ResolveEntrypoint(calledNumber string) (string, bool) {
	if c.Kind == SIPKindEndpoint {
		return c.Entrypoint, c.Entrypoint != ""
	}
	want := NormalizeCalledNumber(calledNumber)
	catchAll := ""
	for _, route := range c.Routes {
		if route.CatchAll {
			catchAll = route.Entrypoint
			continue
		}
		if NormalizeCalledNumber(route.Number) == want {
			return route.Entrypoint, true
		}
	}
	return catchAll, catchAll != ""
}

// Validate checks cross-field invariants that generated Zod types cannot
// express and that the runtime must not trust the editor to enforce.
func (d Definition) Validate() error {
	if len(d.SIP.Connections) == 0 {
		return fmt.Errorf("sip.connection: at least one connection is required")
	}
	ids := make(map[string]bool)
	ports := make(map[string]string)
	for i, c := range d.SIP.Connections {
		where := fmt.Sprintf("sip.connection[%d]", i)
		if !sipConfigIDPattern.MatchString(c.ID) {
			return fmt.Errorf("%s.id must use only letters, numbers, underscores, or hyphens", where)
		}
		if ids[c.ID] {
			return fmt.Errorf("%s.id %q is duplicated", where, c.ID)
		}
		ids[c.ID] = true
		if c.Kind != SIPKindEndpoint && c.Kind != SIPKindTrunk {
			return fmt.Errorf("%s.kind must be endpoint or trunk", where)
		}
		if c.Registration != SIPRegistrationActive && c.Registration != SIPRegistrationInbound {
			return fmt.Errorf("%s.registration must be registered or inbound", where)
		}
		if c.Transport == "" {
			c.Transport = "udp"
		}
		switch strings.ToLower(c.Transport) {
		case "udp", "tcp", "ws":
		default:
			return fmt.Errorf("%s.transport must be udp, tcp, or ws", where)
		}
		if c.LocalPort < 0 || c.LocalPort > 65535 {
			return fmt.Errorf("%s.local_port must be between 0 and 65535", where)
		}
		if c.Registration == SIPRegistrationActive {
			if strings.TrimSpace(c.Server) == "" || strings.TrimSpace(c.Extension) == "" {
				return fmt.Errorf("%s registered connections require server and extension", where)
			}
		} else {
			if c.LocalPort == 0 {
				return fmt.Errorf("%s inbound connections require an explicit local_port", where)
			}
			if len(c.AllowedCIDRs) == 0 {
				return fmt.Errorf("%s inbound connections require allowed_cidrs", where)
			}
			if strings.TrimSpace(c.Username) == "" {
				return fmt.Errorf("%s inbound connections require an authentication username", where)
			}
		}
		for _, raw := range c.AllowedCIDRs {
			if _, _, err := net.ParseCIDR(strings.TrimSpace(raw)); err != nil {
				return fmt.Errorf("%s.allowed_cidrs contains invalid CIDR %q", where, raw)
			}
		}
		if c.LocalPort != 0 {
			key := strings.ToLower(c.Transport) + ":" + fmt.Sprint(c.LocalPort)
			if other := ports[key]; other != "" {
				return fmt.Errorf("%s reuses %s already owned by connection %q", where, key, other)
			}
			ports[key] = c.ID
		}

		switch c.Kind {
		case SIPKindEndpoint:
			if _, ok := d.Functions[c.Entrypoint]; !ok {
				return fmt.Errorf("%s.entrypoint %q does not name a function", where, c.Entrypoint)
			}
			if len(c.Routes) != 0 {
				return fmt.Errorf("%s endpoint connections cannot define routes", where)
			}
		case SIPKindTrunk:
			if c.Entrypoint != "" {
				return fmt.Errorf("%s trunk connections use routes, not entrypoint", where)
			}
			if len(c.Routes) == 0 {
				return fmt.Errorf("%s trunk connections require at least one route", where)
			}
			routeIDs := make(map[string]bool)
			numbers := make(map[string]bool)
			catchAll := false
			for j, route := range c.Routes {
				routeWhere := fmt.Sprintf("%s.route[%d]", where, j)
				if !sipConfigIDPattern.MatchString(route.ID) || routeIDs[route.ID] {
					return fmt.Errorf("%s.id must be present and unique", routeWhere)
				}
				routeIDs[route.ID] = true
				if _, ok := d.Functions[route.Entrypoint]; !ok {
					return fmt.Errorf("%s.entrypoint %q does not name a function", routeWhere, route.Entrypoint)
				}
				if route.CatchAll {
					if catchAll {
						return fmt.Errorf("%s defines more than one catch-all", where)
					}
					if strings.TrimSpace(route.Number) != "" {
						return fmt.Errorf("%s catch-all cannot also define number", routeWhere)
					}
					catchAll = true
					continue
				}
				number := NormalizeCalledNumber(route.Number)
				if number == "" || numbers[number] {
					return fmt.Errorf("%s.number must be present and unique after normalization", routeWhere)
				}
				numbers[number] = true
			}
		}
	}
	return nil
}
