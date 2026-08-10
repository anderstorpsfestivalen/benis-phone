package controller

import (
	"testing"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
)

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
			{Name: "bypass"},
		},
		Functions: make(map[string]*functions.Fn),
	}
	def.Prepare()

	session := NewSession("test", "bypass", nil, nil, nil, nil, def, nil)
	session.handleAction(&functions.Action{
		Reuse: functions.ActionReference{Function: "main", Key: 4},
	})
	if !session.activeScript {
		t.Fatal("referenced script did not start")
	}
	select {
	case <-session.scriptDone:
	case <-time.After(time.Second):
		t.Fatal("referenced script did not finish")
	}
}
