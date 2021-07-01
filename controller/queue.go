package controller

import (
	"math/rand"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type Queue struct{}

func (m *Queue) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	//keychan := c.Phone.GetKeyChannel()
	queueSpot := rand.Intn(200)
	changeQueue := time.NewTimer(time.Second * time.Duration(rand.Intn(60)))
	readQueue := time.NewTicker(time.Second * time.Duration(rand.Intn(60)))
	sanity := time.NewTicker(time.Millisecond * 20)

	for {
		select {
		// change queue timer
		case _ = <-changeQueue.C:
			queueSpot = queueSpot - 1
			changeQueue = time.NewTimer(time.Second * time.Duration(rand.Intn(60)))
		// read queue timer
		case _ = <-readQueue.C:
			readQueue = time.NewTicker(time.Second * time.Duration(rand.Intn(60)))
		}

		// Random messages for customer in queue
		switch behv := rand.Intn(100); {
		case behv > 80:
			message := "Ditt samtal är mycket viktigt för oss. Vi behandlar ditt samtal så fort vi kan."
			ttsData, err := c.Polly.TTS(message, "Astrid")
			if err != nil {
				log.Error(err)
			}
			c.Audio.PlayMP3FromStream(ttsData)
		case behv < 20:
			message := "Du vet väl om att du även kan hitta oss på webben? w w w. PUNKT anderstorps festivalen. PUNKT . s e."
			ttsData, err := c.Polly.TTS(message, "Astrid")
			if err != nil {
				log.Error(err)
			}
			c.Audio.PlayMP3FromStream(ttsData)
		case behv > 30 && behv < 60:
			message := "Vi utför en kvalitetsundersökning. Efter att samtalet är slut ber vi dig att inte inte lägga på luren, undersökningen består av 5 frågor och tar mindre än 1 minut."
			ttsData, err := c.Polly.TTS(message, "Astrid")
			if err != nil {
				log.Error(err)
			}
			c.Audio.PlayMP3FromStream(ttsData)
		}

		// Report current position in queue
		message := "Din plats i kön är. " + strconv.Itoa(queueSpot)
		ttsData, err := c.Polly.TTS(message, "Astrid")
		if err != nil {
			log.Error(err)
		}
		c.Audio.PlayMP3FromStream(ttsData)

		// Sanity check
		_ = <-sanity.C

		if c.Where != "queue" {
			readQueue.Stop()
			changeQueue.Stop()
		}
	}
	return MenuReturn{
		NextFunction: "mainmenu",
	}
}
func (m *Queue) InputLength() int {
	return 0
}

func (m *Queue) Name() string {
	return "queue"
}

func (m *Queue) Prefix(c *Controller) {
	message := "Just nu är det många som ringer till oss. Ditt samtal är placerat i kö. Vi besvarar ditt samtal så fort vi kan."
	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		log.Error(err)
	}
	c.Audio.PlayMP3FromStream(ttsData)
}
