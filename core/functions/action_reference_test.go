package functions

import (
	"strings"
	"testing"
)

func reusableActionDefinition() Definition {
	d := Definition{
		SIP: SIPConfig{Connections: []SIPConnection{{
			ID:           "entry",
			Kind:         SIPKindEndpoint,
			Registration: SIPRegistrationActive,
			Server:       "pbx.test:5060",
			Extension:    "7001",
			Entrypoint:   "bypass",
		}}},
		UnsortedFunctions: []Fn{
			{
				Name: "main",
				Actions: []Action{{
					Num:    4,
					Name:   "beer",
					Script: Script{Code: `speak("shared");`},
				}},
			},
			{
				Name: "bypass",
				Actions: []Action{{
					Num:   1,
					Auto:  true,
					Reuse: ActionReference{Function: "main", Key: 4},
				}},
			},
		},
		Functions: make(map[string]*Fn),
	}
	d.Prepare()
	return d
}

func TestResolveReusableAction(t *testing.T) {
	d := reusableActionDefinition()
	if err := d.Validate(); err != nil {
		t.Fatalf("valid reusable action definition: %v", err)
	}

	action, err := d.ResolveActionReference(ActionReference{Function: "main", Key: 4})
	if err != nil {
		t.Fatalf("resolve reusable action: %v", err)
	}
	if action.Name != "beer" || action.Script.Code == "" {
		t.Fatalf("resolved action = %#v; want shared beer script", action)
	}
	if kind, err := d.Functions["bypass"].Actions[0].Type(); err != nil || kind != "reuse" {
		t.Fatalf("reuse action kind = %q, %v; want reuse", kind, err)
	}
}

func TestReusableActionValidationRejectsMissingAndCycles(t *testing.T) {
	missing := reusableActionDefinition()
	missing.Functions["bypass"].Actions[0].Reuse.Function = "missing"
	missing.UnsortedFunctions[1].Actions[0].Reuse.Function = "missing"
	if err := missing.Validate(); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing reference error = %v", err)
	}

	cycle := reusableActionDefinition()
	cycle.UnsortedFunctions[0].Actions[0] = Action{
		Num:   4,
		Reuse: ActionReference{Function: "bypass", Key: 1},
	}
	cycle.Functions = make(map[string]*Fn)
	cycle.Prepare()
	if err := cycle.Validate(); err == nil || !strings.Contains(err.Error(), "cyclic reuse") {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestReusableActionTOMLRoundTrip(t *testing.T) {
	raw := `
[sip]
[[sip.connection]]
id = "entry"
kind = "endpoint"
registration = "registered"
server = "pbx.test:5060"
extension = "7001"
entrypoint = "bypass"

[[fn]]
name = "main"
actions = [{ num = 4, name = "beer", script = { code = "return;" } }]

[[fn]]
name = "bypass"
hangup_on_return = true
actions = [{ num = 1, auto = true, reuse = { fn = "main", key = 4 } }]
`
	d, err := Decode([]byte(raw))
	if err != nil {
		t.Fatalf("decode reusable action: %v", err)
	}
	got := d.Functions["bypass"].Actions[0]
	if !d.Functions["bypass"].HangupOnReturn {
		t.Fatal("decoded bypass menu did not retain hangup_on_return")
	}
	if !got.Auto || got.Reuse.Function != "main" || got.Reuse.Key != 4 {
		t.Fatalf("decoded reusable action = %#v", got)
	}
}
