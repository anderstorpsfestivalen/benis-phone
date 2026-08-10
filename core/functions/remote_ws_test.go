package functions

import (
	"encoding/json"
	"testing"
)

func TestWSWatcherInitialMessagesReplaySnapshot(t *testing.T) {
	w := &WSWatcher{
		InstanceID: "ivr-1",
		Snapshot: func() [][]byte {
			return [][]byte{[]byte(`{"type":"sip-status","connection_id":"primary"}`)}
		},
	}
	messages := w.initialMessages()
	if len(messages) != 2 {
		t.Fatalf("initial message count = %d, want 2", len(messages))
	}
	var hello struct {
		Type       string `json:"type"`
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(messages[0], &hello); err != nil {
		t.Fatalf("decode hello: %v", err)
	}
	if hello.Type != "runtime-hello" || hello.InstanceID != "ivr-1" {
		t.Fatalf("hello = %#v", hello)
	}
	if string(messages[1]) != `{"type":"sip-status","connection_id":"primary"}` {
		t.Fatalf("snapshot payload = %s", messages[1])
	}
}
