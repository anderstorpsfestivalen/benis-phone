package functions

import (
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
	for i, f := range d.UnsortedFunctions {
		f.IndexActions()
		d.UnsortedFunctions[i].Prefix.TTS.SetDefault(d.General.DefaultTTSVoice, d.General.DefaultTTSLanguage)

	}

	for i, v := range d.UnsortedFunctions {
		d.Functions[v.Name] = &d.UnsortedFunctions[i]
	}

	for i, v := range d.UnsortedFunctions {
		for n, a := range v.Actions {
			t, _ := a.Type()
			if t == "tts" {
				d.UnsortedFunctions[i].Actions[n].TTS.SetDefault(d.General.DefaultTTSVoice, d.General.DefaultTTSLanguage)
			}
		}
	}

}

type General struct {
	Entrypoint string

	// https://docs.aws.amazon.com/polly/latest/dg/voicelist.html
	DefaultTTSVoice string `toml:"default_tts_voice"`

	// https://docs.aws.amazon.com/polly/latest/dg/SupportedLanguage.html
	DefaultTTSLanguage string `toml:"default_tts_lang"`
}
