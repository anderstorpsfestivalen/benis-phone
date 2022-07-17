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

	var conf Definition
	_, err = toml.Decode(string(dat), &conf)

	if err != nil {
		return Definition{}, err
	}

	conf.Functions = make(map[string]*Fn)

	conf.Prepare()

	return conf, nil
}

type Definition struct {
	General           General
	UnsortedFunctions []Fn `toml:"fn"`

	Functions map[string]*Fn

	Queues []Queue `toml:"queue"`
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
				)
			}

			if a.Prefix != (Prefix{}) {
				d.UnsortedFunctions[i].Actions[n].Prefix.TTS.SetDefault(
					d.General.DefaultTTSVoice,
					d.General.DefaultTTSLanguage,
					d.General.DefaultTTSEngine,
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
				)
			}
		}

		t, _ := q.EntryMessage.Type()
		if t == "tts" {
			d.Queues[i].EntryMessage.TTS.SetDefault(
				d.General.DefaultTTSVoice,
				d.General.DefaultTTSLanguage,
				d.General.DefaultTTSEngine,
			)

			d.Queues[i].End.TTS.SetDefault(
				d.General.DefaultTTSVoice,
				d.General.DefaultTTSLanguage,
				d.General.DefaultTTSEngine,
			)
		}

		d.Queues[i].CurrentPositionTemplate.SetDefault(
			d.General.DefaultTTSVoice,
			d.General.DefaultTTSLanguage,
			d.General.DefaultTTSEngine,
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
}
