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
	for _, f := range d.UnsortedFunctions {
		f.IndexActions()
	}

	for _, v := range d.UnsortedFunctions {
		d.Functions[v.Name] = &v
	}
}

type General struct {
	Entrypoint string
	DefaultTTS string `toml:"default_tts"`
}

type Prefix struct {
	File string
}

type Action struct {
	Num  int
	Type string `toml:"t"`
	Dst  string
	Wait bool
}
