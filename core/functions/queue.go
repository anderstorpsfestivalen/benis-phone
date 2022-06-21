package functions

import (
	"bytes"
	"math/rand"
	"os"
	"text/template"
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

	//Templates
	CurrentPositionTemplate TTS `toml:"currentpos"`
	curPosTmpl              *template.Template

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
	finish   chan Action

	qt           *time.Timer
	promptTicker *time.Ticker
}

type QueuePositionTemplate struct {
	Position int
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
	Empty  bool
	Weight int
}

func (q *Queue) Load() error {

	q.setDefaults()

	var ch []wr.Choice

	for _, c := range q.Prompts {
		ch = append(ch, wr.NewChoice(c, uint(c.Weight)))
	}

	chooser, err := wr.NewChooser(ch...)

	if err != nil {
		return err
	}

	q.rm = chooser

	return nil
}

func (q *Queue) Start(audio *audio.Audio, polly polly.Polly) <-chan Action {
	q.a = audio
	q.p = polly
	q.kill = make(chan bool)
	q.finish = make(chan Action)

	ttmpl, err := template.New("queueposition").Parse(q.CurrentPositionTemplate.Message)
	if err != nil {
		log.Error(err)
	}
	q.curPosTmpl = ttmpl

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
	q.startBackground()

	// prep for main queue loop
	q.queueSpot = rand.Intn(
		rand.Intn(q.MaxQueuePositon-q.MinQueuePosition) + q.MinQueuePosition)

	q.qt = q.queueTimer()
	q.promptTicker = time.NewTicker(4)

	// Main queue loop
	for {
		select {
		// Stop the queue (hangup etc)
		case <-q.kill:
			q.qt.Stop()
			q.promptTicker.Stop()
			q.finish <- Action{}

			return

		// Decrease the queue
		case <-q.qt.C:

			q.queueSpot = q.queueSpot - 1

			if q.queueSpot < 1 {
				go q.EndOfLine()
				return
			}

			q.qt = q.queueTimer()

		case <-q.promptTicker.C:

			q.prompt()
		}
	}
}

func (q *Queue) startBackground() {
	f, _ := os.Open(q.BackgroundMusic.Source)

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		log.Error(err)
	}
	q.streamer = streamer
	q.streamer.Seek(q.lastPos)

	go q.a.ExternalPlayback(q.streamer, format)

}

func (q *Queue) pauseBackground() {

	if q.streamer != nil {
		if q.streamer.Position()-40 >= q.streamer.Len() {
			q.lastPos = 0
		} else {
			q.lastPos = q.streamer.Position()
		}
		q.a.Clear()
	}
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

func (q *Queue) prompt() {

	// Pause background music
	q.pauseBackground()

	// Read current position
	// Generate message from template
	ds := QueuePositionTemplate{
		Position: q.queueSpot,
	}

	posMsg := new(bytes.Buffer)
	err := q.curPosTmpl.Execute(posMsg, ds)
	if err != nil {
		log.Error(err)
	}

	// Read it out with tts
	qt, _ := TTS{
		Message:  posMsg.String(),
		Voice:    q.CurrentPositionTemplate.Voice,
		Language: q.CurrentPositionTemplate.Language,
	}.GetPlayable()

	qt.Wait = true

	qt.Play(q.a, q.p)

	// Get random message to read out
	p := q.rm.Pick().(QueuePrompt)
	if !p.Empty {
		p.Prompt.Wait = true
		p.Prompt.Play(q.a, q.p)
	}

	q.startBackground()

	q.promptTicker = q.readTicker(q.promptTicker)
}

// If you get here hahaha WHAT A WASTE
func (q *Queue) EndOfLine() {
	q.qt.Stop()
	q.promptTicker.Stop()
	q.finish <- q.End
}
