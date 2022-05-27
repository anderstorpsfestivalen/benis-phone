package controller

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	log "github.com/sirupsen/logrus"
)

type Queue struct {
	lastPos  int
	streamer beep.StreamSeekCloser
}

func (m *Queue) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	rand.Seed(time.Now().UTC().UnixNano())
	message := "Just nu är det många som ringer till oss. Ditt samtal är placerat i kö. Vi besvarar ditt samtal så fort vi kan."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)

	//keychan := c.Phone.GetKeyChannel()
	sub := c.Subscribe(m.Name())
	queueSpot := rand.Intn(rand.Intn(60-20) + 20)
	changeQueue := time.NewTimer(time.Second * time.Duration(rand.Intn(60)))
	readQueue := time.NewTicker(4)
	sanity := time.NewTicker(time.Millisecond * 20)

	for {
		select {
		//hang up
		case <-sub.Cancel:
			m.streamer = nil
			readQueue.Stop()
			changeQueue.Stop()
			c.Unsubscribe(m.Name())
			return MenuReturn{
				NextFunction: "mainmenu",
			}
		// change queue timer
		case <-changeQueue.C:
			queueSpot = queueSpot - 1
			if queueSpot < 1 {
				message := "Tekniskt fel, vi kan tyvärr inte ta emot ditt samtal. Var god försök igen."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
				return MenuReturn{
					NextFunction: "mainmenu",
				}
			}
			changeQueue = time.NewTimer(time.Second * time.Duration(rand.Intn(60)+1))
		// read queue timer
		case <-readQueue.C:
			m.pauseBackground(c)
			readQueue = time.NewTicker(time.Second * time.Duration(rand.Intn(120-35)+35))

			// Report current position in queue
			message := "Din plats i cön är: " + strconv.Itoa(queueSpot)
			ttsData, err := c.Polly.TTS(message, "Astrid")
			if err != nil {
				log.Error(err)
			}
			c.Audio.PlayMP3FromStream(ttsData)

			// Random messages for customer in queue
			switch behv := rand.Intn(100); {
			case behv > 80:
				message := "Ditt samtal är mycket viktigt för oss. Vi behandlar ditt samtal så fort vi kan."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
			case behv < 10:
				message := "Du vet väl om att du även kan hitta oss på webben? w w w. PUNKT anderstorps festivalen. PUNKT . s. e."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
			case behv > 10 && behv < 20:
				message := "Visste du att du kan få svar på många frågor genom att besöka vår hemsida? w w w. PUNKT anderstorps festivalen. PUNKT . s. e."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
			case behv > 30 && behv < 60:
				message := "Vi utför en kvalitetsundersökning. Efter att samtalet är slut ber vi dig att inte lägga på luren, undersökningen består av 5 frågor och tar mindre än en minut."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
			}

			m.startBackground(c)
		}
		// Sanity check
		_ = <-sanity.C

		if c.Where != "queue" {
			readQueue.Stop()
			changeQueue.Stop()
		}
	}
}
func (m *Queue) InputLength() int {
	return 0
}

func (m *Queue) Name() string {
	return "queue"
}

func (m *Queue) Prefix(c *Controller) {
}

func (m *Queue) pauseBackground(c *Controller) {

	if m.streamer != nil {
		if m.streamer.Position()-40 >= m.streamer.Len() {
			m.lastPos = 0
		} else {
			m.lastPos = m.streamer.Position()
		}
		c.Audio.Clear()
	}
}

func (m *Queue) startBackground(c *Controller) {
	f, _ := os.Open("files/hold.mp3")

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Println("penis")
	}
	m.streamer = streamer
	m.streamer.Seek(m.lastPos)

	go c.Audio.ExternalPlayback(m.streamer, format)

}
