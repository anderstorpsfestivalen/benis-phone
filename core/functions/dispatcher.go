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

type EmptyDispatcher struct {
}

func (e *EmptyDispatcher) Load() error {
	return nil
}

func (q *EmptyDispatcher) Start(audio *audio.Audio, rec *audio.Recorder, polly polly.Polly) <-chan Action {
	return make(<-chan Action)
}

func (q *EmptyDispatcher) Stop() {

}
