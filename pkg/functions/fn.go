package functions

import (
	"fmt"
	"strconv"
)

type Fn struct {
	Name           string
	Prefix         Prefix
	ClearCallstack bool `toml:"clear_callstack"`
	InputLength    int

	Actions []Action
}

func (f *Fn) IndexActions() {
	for i, val := range f.Actions {
		if val.Num == 0 {
			f.Actions[i].Num = i + 1
		}
	}
}

func (f *Fn) Enter() {

}

func (f *Fn) ResolveAction(key string) (*Action, error) {

	if key == "*" {
		key = "10"
	}
	if key == "#" {
		key = "11"
	}

	num, err := strconv.Atoi(key)
	if err != nil {
		return nil, err
	}

	if num < 0 && num > 11 {
		return nil, fmt.Errorf("number outside of range (0-9, * (10), # (11)), WTF did you do?")
	}

	l := Action{}
	found := false
	for _, a := range f.Actions {
		if a.Num == num {
			l = a
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("could not find key")
	}

	return &l, nil
}
