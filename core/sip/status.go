package sip

import "time"

// StatusEvent is safe to send to the control plane. Message must never
// contain credentials or Authorization headers.
type StatusEvent struct {
	ConnectionID string    `json:"connection_id"`
	State        string    `json:"state"`
	Code         string    `json:"code,omitempty"`
	Message      string    `json:"message,omitempty"`
	LocalPort    int       `json:"local_port,omitempty"`
	At           time.Time `json:"at"`
}

func newStatus(connectionID, state, code, message string, port int) StatusEvent {
	return StatusEvent{
		ConnectionID: connectionID,
		State:        state,
		Code:         code,
		Message:      message,
		LocalPort:    port,
		At:           time.Now().UTC(),
	}
}
