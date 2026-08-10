package controller

import (
	"context"
	"testing"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
)

type returnCallControl struct {
	hangups int
}

func (c *returnCallControl) ID() string { return "test" }
func (c *returnCallControl) Hangup(context.Context) error {
	c.hangups++
	return nil
}
func (c *returnCallControl) Transfer(context.Context, string) error { return nil }
func (c *returnCallControl) StartRecording(string) (string, error)  { return "", nil }
func (c *returnCallControl) StopRecording() error                   { return nil }
func (c *returnCallControl) SendDTMF(string) error                  { return nil }

func TestHandleReusableActionRunsSourceScript(t *testing.T) {
	def := functions.Definition{
		UnsortedFunctions: []functions.Fn{
			{
				Name: "main",
				Actions: []functions.Action{{
					Num:    4,
					Name:   "shared",
					Script: functions.Script{Code: `return;`},
				}},
			},
			{
				Name:           "bypass",
				HangupOnReturn: true,
				Actions: []functions.Action{{
					Auto:  true,
					Reuse: functions.ActionReference{Function: "main", Key: 4},
				}},
			},
		},
		Functions: make(map[string]*functions.Fn),
	}
	def.Prepare()

	callControl := &returnCallControl{}
	session := NewSession("test", "bypass", nil, &fakeSink{}, nil, nil, def, callControl)
	session.HookState = true
	if err := session.enterFunction("bypass"); err != nil {
		t.Fatalf("enter bypass function: %v", err)
	}
	if current := session.getCurrent().Name; current != "bypass" {
		t.Fatalf("reused action changed current function to %q; want bypass", current)
	}
	if err := session.handlePrefix(); err != nil {
		t.Fatalf("empty bypass prefix must be a no-op: %v", err)
	}
	if !session.activeScript {
		t.Fatal("referenced script did not start")
	}
	var dst string
	select {
	case dst = <-session.scriptDone:
	case <-time.After(time.Second):
		t.Fatal("referenced script did not finish")
	}
	session.handleScriptDone(dst)
	if callControl.hangups != 1 {
		t.Fatalf("hangups = %d; want 1 after reused script returns to bypass", callControl.hangups)
	}
}
