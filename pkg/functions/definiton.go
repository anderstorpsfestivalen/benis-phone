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
}

func (d *Definition) Prepare() {
	for i, f := range d.UnsortedFunctions {
		f.IndexActions()
		d.UnsortedFunctions[i].Prefix.TTS.SetDefault(d.General.DefaultTTSVoice, d.General.DefaultTTSLanguage)

	}

	for i, v := range d.UnsortedFunctions {
		d.Functions[v.Name] = &d.UnsortedFunctions[i]
	}

}

type General struct {
	Entrypoint string

	// https://docs.aws.amazon.com/polly/latest/dg/voicelist.html
	DefaultTTSVoice string `toml:"default_tts_voice"`

	// https://docs.aws.amazon.com/polly/latest/dg/SupportedLanguage.html
	DefaultTTSLanguage string `toml:"default_tts_lang"`
}

type Action struct {
	Num  int
	Type string `toml:"t"`
	Dst  string
	Wait bool
}
