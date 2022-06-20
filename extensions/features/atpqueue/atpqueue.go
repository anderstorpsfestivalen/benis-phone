package atpqueue

import (
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/faiface/beep"
)

type ATPQueue struct {
	lastPos  int
	streamer beep.StreamSeekCloser
}

func (f *ATPQueue) Start(audio *audio.Audio, polly polly.Polly) {
	f.reset()

	message := `Just nu är det många som ringer till oss. 
	Ditt samtal är placerat i kö.
	Vi besvarar ditt samtal så fort vi kan.`

}

func (f *ATPQueue) Input(key string) {

}

func (f *ATPQueue) Stop() {

}

func (f *ATPQueue) reset() {
	f.lastPos = 0
	f.streamer = nil

}
