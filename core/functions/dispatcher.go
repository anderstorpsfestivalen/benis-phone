package functions

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
)

type Dispatcher interface {
	Load() error
	Start(audio *audio.Audio, rec *audio.Recorder, polly polly.Polly) <-chan Action
	Stop()
}

var Dispatchers = map[string]Dispatcher{}
