package controller

import (
	"math/rand"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type Queue struct{}

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
	queueSpot := rand.Intn(200)
	changeQueue := time.NewTimer(time.Second * time.Duration(rand.Intn(60)))
	readQueue := time.NewTicker(4)
	sanity := time.NewTicker(time.Millisecond * 20)

	for {
		select {
		//hang up
		case <-sub.Cancel:
			c.Unsubscribe(m.Name())
			return MenuReturn{
				NextFunction: menu.Caller,
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
					NextFunction: menu.Caller,
				}
			}
			changeQueue = time.NewTimer(time.Second * time.Duration(rand.Intn(60)+1))
		// read queue timer
		case <-readQueue.C:
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
				message := "Du vet väl om att du även kan hitta oss på webben? w w w. PUNKT anderstorps festivalen. PUNKT . s e."
				ttsData, err := c.Polly.TTS(message, "Astrid")
				if err != nil {
					log.Error(err)
				}
				c.Audio.PlayMP3FromStream(ttsData)
			case behv > 10 && behv < 20:
				message := "Visste du att du kan få svar på många frågor genom att besöka vår hemsida? w w w. PUNKT anderstorps festivalen. PUNKT . s e."
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

			go c.Audio.PlayFromFile("files/hold.mp3")
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

func (m *Queue) pauseBackground() {

}

func (m *Queue) startBackground() {

}
