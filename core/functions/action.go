package functions

import "fmt"

type Action struct {
	Num   int
	Wait  bool
	Clear bool

	//////////////
	// actionables
	//////////////

	// Play something before triggering the action
	Prefix Prefix `toml:"prefix"`
	// Links to another menu
	Dst string
	// Plays a file (mp3, ogg, etc)
	File File `toml:"file"`
	// Plays a random file from a folder
	RandomFile RandomFile `toml:"randomfile"`
	// Reads a TTS
	TTS TTS `toml:"tts"`
	// Calls a service
	Service Service `toml:"srv"`
	// Redirects to a custom dispatcher
	CustomDispatcher string `toml:"dispatcher"`

	// Call control (SIP mode). Each is mutually exclusive with the audio
	// actions above; exactly one action type per Action.
	//
	// Transfer issues a blind REFER. Value: full SIP URI
	// ("sip:200@host"), "user@host", or extension shorthand ("200")
	// which expands using the configured SIP domain.
	Transfer string `toml:"transfer"`
	// Hangup terminates the call when true.
	Hangup bool `toml:"hangup"`
	// Record controls per-call recording. "start" or "stop".
	Record string `toml:"record"`
	// RecordTo is the subfolder under SIP.RecordPath when Record=="start".
	// Defaults to "adhoc".
	RecordTo string `toml:"record_to"`
	// DTMF transmits these digits to the remote party (RFC 4733), with
	// 200 ms between each.
	DTMF string `toml:"dtmf"`
}

func (a *Action) Type() (string, error) {
	if a.Dst != "" {
		return "fn", nil
	}

	if a.File != (File{}) {
		return "file", nil
	}

	if a.RandomFile != (RandomFile{}) {
		return "randomfile", nil
	}

	if a.TTS != (TTS{}) {
		return "tts", nil
	}

	if a.Service.Destination != "" {
		return "srv", nil
	}

	if a.CustomDispatcher != "" {
		return "dispatcher", nil
	}

	if a.Transfer != "" {
		return "transfer", nil
	}

	if a.Hangup {
		return "hangup", nil
	}

	if a.Record != "" {
		return "record", nil
	}

	if a.DTMF != "" {
		return "dtmf", nil
	}

	if a.Clear {
		return "clear", nil
	}

	return "", fmt.Errorf("cannot determine action type")
}

func (a *Action) GetPrefix() (Prefix, error) {
	if a.Prefix != (Prefix{}) {
		return a.Prefix, nil
	}

	return Prefix{}, fmt.Errorf("no prefix")
}
