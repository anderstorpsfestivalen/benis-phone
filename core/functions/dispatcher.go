package functions

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
)

type Dispatcher interface {
	Start(audio *audio.Audio, polly polly.Polly) <-chan bool
	Stop()
}
