package sip

import (
	"context"
	"errors"
	"testing"
)

type hangupCloserStub struct {
	hangupErr error
	closeErr  error
	hungUp    bool
	closed    bool
}

func (d *hangupCloserStub) Hangup(context.Context) error {
	d.hungUp = true
	return d.hangupErr
}

func (d *hangupCloserStub) Close() error {
	d.closed = true
	return d.closeErr
}

func TestHangupAndCloseReleasesLocalDialog(t *testing.T) {
	dialog := &hangupCloserStub{}

	if err := hangupAndClose(context.Background(), dialog); err != nil {
		t.Fatalf("hangup and close: %v", err)
	}
	if !dialog.hungUp {
		t.Fatal("SIP hangup was not sent")
	}
	if !dialog.closed {
		t.Fatal("local dialog was not closed")
	}
}

func TestHangupAndCloseStillClosesAfterHangupError(t *testing.T) {
	hangupErr := errors.New("hangup failed")
	dialog := &hangupCloserStub{hangupErr: hangupErr}

	err := hangupAndClose(context.Background(), dialog)
	if !errors.Is(err, hangupErr) {
		t.Fatalf("error = %v, want hangup error", err)
	}
	if !dialog.closed {
		t.Fatal("local dialog was not closed after SIP hangup failed")
	}
}
