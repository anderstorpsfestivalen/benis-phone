package controller

import (
	"testing"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
)

func testDefinition(names ...string) functions.Definition {
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

func TestCreateSessionUsesListenerDefinitionSnapshot(t *testing.T) {
	oldDef := testDefinition("old-entrypoint")
	newDef := testDefinition("new-entrypoint")
	manager := NewSessionManager(nil, oldDef, 10)
	manager.UpdateDefinition(newDef)

	session, err := manager.CreateSession(
		"call-1", "old-entrypoint", oldDef, nil, nil, nil, nil,
	)
	if err != nil {
		t.Fatalf("create session from listener snapshot: %v", err)
	}
	if session.Definition.Functions["old-entrypoint"] == nil {
		t.Fatal("session did not retain listener definition snapshot")
	}
	if session.Definition.Functions["new-entrypoint"] != nil {
		t.Fatal("session unexpectedly used manager's newer definition")
	}
}
