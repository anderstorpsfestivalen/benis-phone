// Package callctl provides an abstraction for per-call control operations
// (hangup, transfer, recording, DTMF send). One Controller is wired into
// each controller.Session by the SIP server, so handlers can reach call
// control without depending on the SIP layer directly. Local-audio mode
// leaves the Controller nil; handlers check before using.
package callctl

import (
	"context"
	"errors"
)

// Controller is the contract every call-control backend implements.
type Controller interface {
	// ID returns the call identifier (for logging).
	ID() string

	// Hangup terminates the call. Sends BYE if confirmed, otherwise a 486.
	Hangup(ctx context.Context) error

	// Transfer issues a blind REFER asking the remote endpoint to redirect
	// to target. After the PBX accepts, the original call ends.
	//
	// target may be a full SIP URI ("sip:200@host[:port]"), a bare URI
	// without scheme ("200@host"), or an extension shorthand ("200") in
	// which case it expands to "sip:200@<default-domain>".
	Transfer(ctx context.Context, target string) error

	// StartRecording begins writing both legs of RTP to a WAV under
	// <base>/<subfolder>/<timestamp>.wav. Returns the absolute path of
	// the recording, or ErrAlreadyRecording if one is already in progress.
	StartRecording(subfolder string) (path string, err error)

	// StopRecording finalizes the current recording. Returns ErrNotRecording
	// if no recording is active.
	StopRecording() error

	// SendDTMF transmits one or more DTMF tones (RFC 4733) with a 200 ms
	// gap between digits. Valid characters: 0-9, *, #, A-D.
	SendDTMF(digits string) error
}

var (
	ErrAlreadyRecording = errors.New("callctl: already recording")
	ErrNotRecording     = errors.New("callctl: not recording")
)
