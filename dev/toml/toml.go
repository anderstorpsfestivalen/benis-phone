package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Definition struct {
	General   General
	Functions []Fn `toml:"fn"`
}

func (d *Definition) Normalize() {
	for _, f := range d.Functions {
		f.IndexActions()
	}
}

type General struct {
	Entrypoint string
	DefaultTTS string `toml:"default_tts"`
}

type Fn struct {
	Name        string
	Prefix      Prefix
	Exit        string
	InputLength int

	Actions []Action
}

func (f *Fn) IndexActions() {
	for i, val := range f.Actions {
		if val.Num == 0 {
			f.Actions[i].Num = i
		}
	}
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

func main() {
	dat, err := os.ReadFile("definitions/def.toml")
	if err != nil {
		panic(err)
	}

	var conf Definition
	_, err = toml.Decode(string(dat), &conf)

	if err != nil {
		panic(err)
	}

	conf.Normalize()

	fmt.Println(conf)
}
