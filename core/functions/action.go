package functions

import "fmt"

type Action struct {
	Num   int
	Wait  bool
	Clear bool

	// Name is a human-readable label for this action node. It is stored
	// and round-tripped through the config so authors can see at a glance
	// what each node does in the editor's graph, but the runtime ignores
	// it — purely a UI/documentation aid.
	Name string `toml:"name"`

	//////////////
	// actionables
	//////////////

	// Play something before triggering the action (sequential: blocks until done,
	// then the action runs).
	Prefix Prefix `toml:"prefix"`
	// Play something while the action runs (parallel: action work starts
	// immediately, pmsg plays out, action result is held until pmsg finishes).
	// Useful for slow service calls — give the caller something to listen to
	// while svc.Get + TTS synthesis happen in the background.
	Pmsg Prefix `toml:"pmsg"`
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

	// LiveFeed streams audio from a host capture device (microphone, audio
	// interface input) directly into the call's outbound RTP. Runs until
	// the caller presses another digit (which Clear()s it). nil means this
	// action is not a livefeed.
	LiveFeed *LiveFeed `toml:"livefeed"`

	// GenericJSON fetches a configurable HTTP(S) endpoint, decodes the JSON
	// response, renders a Go text/template against it, and speaks the
	// result through TTS. See Type() for how this variant is discovered.
	GenericJSON GenericJSON `toml:"genericjson"`
}

// LiveFeed configures a livefeed action: pick a host capture device by name
// (substring match, case-insensitive) and choose which input channel of that
// device to stream.
type LiveFeed struct {
	// Device is matched as a case-insensitive substring against the names
	// returned by the OS. Empty string opens the system default device.
	Device string `toml:"device"`
	// Channel is the 0-indexed channel within the device to stream. For a
	// stereo interface, 0 = left/input-1, 1 = right/input-2.
	Channel int `toml:"channel"`
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

	if a.LiveFeed != nil {
		return "livefeed", nil
	}

	// GenericJSON is discovered by a non-empty URL: the other fields
	// (template, headers, body) are all meaningful only in combination
	// with a URL, so URL is the canonical discriminator. Matches the TS
	// actionKind() in ui/src/generated/config.ts.
	if a.GenericJSON.URL != "" {
		return "genericjson", nil
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

// HasPmsg reports whether this action has a parallel "playing while running"
// message configured.
func (a *Action) HasPmsg() bool {
	return a.Pmsg != (Prefix{})
}
