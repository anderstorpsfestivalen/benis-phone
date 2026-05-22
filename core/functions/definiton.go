package functions

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

func LoadFromFile(path string) (Definition, error) {
	dat, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, err
	}
	return Decode(dat)
}

// Decode parses raw TOML bytes into a Definition and runs Prepare(). Shared
// by the file loader and the remote loader so they produce identical
// in-memory state.
func Decode(data []byte) (Definition, error) {
	var conf Definition
	if _, err := toml.Decode(string(data), &conf); err != nil {
		return Definition{}, err
	}
	conf.Functions = make(map[string]*Fn)
	conf.Prepare()
	return conf, nil
}

type Definition struct {
	General           General
	SIP               SIPConfig `toml:"sip"`
	UnsortedFunctions []Fn      `toml:"fn"`

	Functions map[string]*Fn

	Queues []Queue `toml:"queue"`
}

// SIPConfig holds SIP client configuration for registering with a PBX.
type SIPConfig struct {
	// Server is the SIP server/PBX address (e.g., "pbx.example.com" or "192.168.1.100:5060")
	Server string `toml:"server"`

	// Extension is the extension number to register as (e.g., "100")
	Extension string `toml:"extension"`

	// Username for SIP authentication (often same as extension)
	Username string `toml:"username"`

	// Domain is the SIP domain (often same as server, e.g., "pbx.example.com")
	Domain string `toml:"domain"`

	// Transport is the SIP transport: udp, tcp, ws, wss (default: "udp")
	Transport string `toml:"transport"`

	// LocalPort is the local port to bind to (default: 5060)
	LocalPort int `toml:"local_port"`

	// MaxConcurrentCalls limits concurrent calls (default: 10)
	MaxConcurrentCalls int `toml:"max_concurrent_calls"`

	// RecordPath is the base directory for call recordings
	RecordPath string `toml:"record_path"`

	// ExpirySeconds is the registration expiry time (default: 300)
	ExpirySeconds int `toml:"expiry_seconds"`

	// ExternalIP is your public IP address for NAT traversal (required if behind NAT)
	// This IP will be used in SDP for RTP media. If empty, local IP is used.
	ExternalIP string `toml:"external_ip"`

	// Direct enables server-less debug mode: bind a listener and accept any
	// unauthenticated INVITE without registering with a PBX. Server and
	// Password are ignored in this mode. Intended for local softphone testing
	// (call sip:anything@<host>:<port>).
	Direct bool `toml:"direct"`

	// Password should be in creds.json under SIP.Password for security
}

func (d *Definition) Prepare() {

	// This was a bad design decision btw.
	// The right way is prob for the controller to keep certain "globals"
	// and pass around that object rather than pre-hydrate all unset variables

	// Hydrate prefixes
	for i, f := range d.UnsortedFunctions {
		f.IndexActions()
		d.UnsortedFunctions[i].Prefix.TTS.SetDefault(
			d.General.DefaultTTSVoice,
			d.General.DefaultTTSLanguage,
			d.General.DefaultTTSEngine,
			d.General.DefaultTTSProvider,
		)

	}

	// Map unsorted functions into map[string]
	for i, v := range d.UnsortedFunctions {
		d.Functions[v.Name] = &d.UnsortedFunctions[i]
	}

	// Hydrate Actions
	for i, v := range d.UnsortedFunctions {
		for n, a := range v.Actions {
			t, _ := a.Type()
			if t == "tts" {
				d.UnsortedFunctions[i].Actions[n].TTS.SetDefault(
					d.General.DefaultTTSVoice,
					d.General.DefaultTTSLanguage,
					d.General.DefaultTTSEngine,
					d.General.DefaultTTSProvider,
				)
			}

			if a.Prefix != (Prefix{}) {
				d.UnsortedFunctions[i].Actions[n].Prefix.TTS.SetDefault(
					d.General.DefaultTTSVoice,
					d.General.DefaultTTSLanguage,
					d.General.DefaultTTSEngine,
					d.General.DefaultTTSProvider,
				)
			}

			if a.Pmsg != (Prefix{}) {
				d.UnsortedFunctions[i].Actions[n].Pmsg.TTS.SetDefault(
					d.General.DefaultTTSVoice,
					d.General.DefaultTTSLanguage,
					d.General.DefaultTTSEngine,
					d.General.DefaultTTSProvider,
				)
			}
		}
	}

	// Hydrate queues
	for i, q := range d.Queues {
		for n, m := range q.Prompts {
			t, _ := m.Prompt.Type()
			if t == "tts" {
				d.Queues[i].Prompts[n].Prompt.TTS.SetDefault(
					d.General.DefaultTTSVoice,
					d.General.DefaultTTSLanguage,
					d.General.DefaultTTSEngine,
					d.General.DefaultTTSProvider,
				)
			}
		}

		t, _ := q.EntryMessage.Type()
		if t == "tts" {
			d.Queues[i].EntryMessage.TTS.SetDefault(
				d.General.DefaultTTSVoice,
				d.General.DefaultTTSLanguage,
				d.General.DefaultTTSEngine,
				d.General.DefaultTTSProvider,
			)

			d.Queues[i].End.TTS.SetDefault(
				d.General.DefaultTTSVoice,
				d.General.DefaultTTSLanguage,
				d.General.DefaultTTSEngine,
				d.General.DefaultTTSProvider,
			)
		}

		d.Queues[i].CurrentPositionTemplate.SetDefault(
			d.General.DefaultTTSVoice,
			d.General.DefaultTTSLanguage,
			d.General.DefaultTTSEngine,
			d.General.DefaultTTSProvider,
		)

	}

}

func (d *Definition) ResolveDispatcher(name string) (Dispatcher, error) {
	for _, q := range d.Queues {
		if q.Name == name {
			return &q, nil
		}
	}

	return &EmptyDispatcher{}, fmt.Errorf("could not find queue %v", name)
}

type General struct {
	Entrypoint string

	// https://docs.aws.amazon.com/polly/latest/dg/voicelist.html
	DefaultTTSVoice string `toml:"default_tts_voice"`

	// https://docs.aws.amazon.com/polly/latest/dg/SupportedLanguage.html
	DefaultTTSLanguage string `toml:"default_tts_lang"`

	// standard, neural
	DefaultTTSEngine string `toml:"default_tts_engine"`

	// "polly" or "elevenlabs"; falls back to the registry's default when empty.
	// Individual menus can override per-TTS via `provider = "..."`.
	DefaultTTSProvider string `toml:"default_tts_provider"`
}
