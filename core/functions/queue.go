package functions

import (
	"math/rand"
	"os"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	wr "github.com/mroth/weightedrand"
	log "github.com/sirupsen/logrus"
)

type Queue struct {
	Name string

	EntryMessage Playable `toml:"entrymsg"`

	MinQueuePosition int `toml:"minpos"`
	MaxQueuePositon  int `toml:"maxpos"`
	QueueSpeed       int `toml:"speed"`

	MinPromptTime int `toml:"minprompt"`
	MaxPromptTime int `toml:"maxprompt"`

	Prompts         []QueuePrompt `toml:"prompt"`
	BackgroundMusic File          `toml:"bgmusic"`
	End             Action

	queueSpot int

	rm       *wr.Chooser
	lastPos  int
	streamer beep.StreamSeekCloser
	a        *audio.Audio
	p        polly.Polly
	kill     chan bool
	finish   chan bool
	rTick    *time.Ticker
}

// I miss rust :( Traits, esp the Default trait is so OP
func (q *Queue) setDefaults() {

	if q.MinQueuePosition == 0 {
		q.MinQueuePosition = 20
		log.Trace("MinQueuePosition default to ", q.MinQueuePosition)
	}

	if q.MaxQueuePositon == 0 {
		q.MaxQueuePositon = 60
		log.Trace("MaxQueuePositon default to ", q.MaxQueuePositon)
	}

	if q.QueueSpeed == 0 {
		q.QueueSpeed = 60
		log.Trace("QueueSpeed default to ", q.QueueSpeed)
	}

	if q.MinPromptTime == 0 {
		q.MinPromptTime = 35
		log.Trace("MinPromptTime default to ", q.MinPromptTime)
	}

	if q.MaxPromptTime == 0 {
		q.MaxPromptTime = 120
		log.Trace("MaxPromptTime default to ", q.MaxPromptTime)
	}

}

type QueuePrompt struct {
	Prompt Playable `toml:"prompt"`
	Weight int
}

func (q *Queue) Load() error {

	q.setDefaults()

	var ch []wr.Choice

	for _, c := range q.Prompts {
		ch = append(ch, wr.NewChoice(c.Prompt, uint(c.Weight)))
	}

	chooser, err := wr.NewChooser(ch...)

	if err != nil {
		return err
	}

	q.rm = chooser

	return nil
}

func (q *Queue) Start(audio *audio.Audio, polly polly.Polly) <-chan bool {
	q.a = audio
	q.p = polly
	q.kill = make(chan bool)
	q.finish = make(chan bool)
	// fmt.Println(rand.Intn(max - min) + min)
	go q.loop()

	return q.finish
}

func (q *Queue) Stop() {
	q.kill <- true
}

func (q *Queue) loop() {

	// play entry message
	q.EntryMessage.Play(q.a, q.p)

	// start background audio
	q.StartBackground(q.a)

	// prep for main queue loop
	q.queueSpot = rand.Intn(
		rand.Intn(q.MaxQueuePositon-q.MinQueuePosition) + q.MinQueuePosition)

	qTimer := q.queueTimer()
	q.rTick = time.NewTicker(4)

	// Main queue loop
	for {
		select {
		// Stop the queue (hangup etc)
		case <-q.kill:
			qTimer.Stop()
			q.rTick.Stop()

			q.finish <- true
			return

		// Decrease the queue
		case <-qTimer.C:

			q.queueSpot = q.queueSpot - 1

			qTimer = q.queueTimer()

		case <-q.rTick.C:

		}
	}
}

func (q *Queue) StartBackground(a *audio.Audio) {
	f, _ := os.Open(q.BackgroundMusic.Source)

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Error(err)
	}
	q.streamer = streamer
	q.streamer.Seek(q.lastPos)

	go a.ExternalPlayback(q.streamer, format)

}

func (q *Queue) queueTimer() *time.Timer {
	return time.NewTimer(time.Second * time.Duration(rand.Intn(q.QueueSpeed)))
}

func (q *Queue) readTicker(oldTimer *time.Ticker) *time.Ticker {
	// Doing this so I don't forget to clear the old one
	// Probably a memory leak in the old code
	// I don't think go realizes it needs to garbage collect tickers that ticks forever
	oldTimer.Stop()
	return time.NewTicker(
		time.Second * time.Duration(
			rand.Intn(q.MaxPromptTime-q.MinPromptTime)+q.MinPromptTime))
}

// If you get here hahaha WHAT A WASTE
func (q *Queue) EndOfLine() {

}
