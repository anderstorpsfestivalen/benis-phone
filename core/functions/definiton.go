package functions

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
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
	if err := conf.Validate(); err != nil {
		return Definition{}, err
	}
	return conf, nil
}

type Definition struct {
	General           General
	SIP               SIPConfig `toml:"sip"`
	UnsortedFunctions []Fn      `toml:"fn"`

	Functions map[string]*Fn

	Queues []Queue `toml:"queue"`
}

// SIPConfig holds process-wide SIP settings and the independently managed
// connections that feed calls into this Definition's function graph.
type SIPConfig struct {
	MaxConcurrentCalls int             `toml:"max_concurrent_calls"`
	RecordPath         string          `toml:"record_path"`
	Connections        []SIPConnection `toml:"connection"`
}

// SIPConnection is one isolated SIP listener. Registered connections send
// REGISTER to Server; inbound connections require Digest auth and AllowedCIDRs.
type SIPConnection struct {
	ID            string   `toml:"id"`
	Name          string   `toml:"name"`
	Kind          string   `toml:"kind"`         // endpoint | trunk
	Registration  string   `toml:"registration"` // registered | inbound
	Server        string   `toml:"server"`
	Extension     string   `toml:"extension"`
	Username      string   `toml:"username"`
	Domain        string   `toml:"domain"`
	Transport     string   `toml:"transport"`
	LocalPort     int      `toml:"local_port"`
	ExpirySeconds int      `toml:"expiry_seconds"`
	ExternalIP    string   `toml:"external_ip"`
	AllowedCIDRs  []string `toml:"allowed_cidrs"`

	Entrypoint string     `toml:"entrypoint"`
	Routes     []SIPRoute `toml:"route"`
}

type SIPRoute struct {
	ID         string `toml:"id"`
	Number     string `toml:"number"`
	Entrypoint string `toml:"entrypoint"`
	CatchAll   bool   `toml:"catch_all"`
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

			// Pre-compile script actions so a syntax error surfaces now (at
			// config load / hot-reload) instead of on the first live call.
			if a.Script.Code != "" {
				if err := d.UnsortedFunctions[i].Actions[n].Script.Compile(); err != nil {
					log.WithFields(log.Fields{
						"fn":     v.Name,
						"action": a.Name,
					}).Warnf("script compile error: %v", err)
				}
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
