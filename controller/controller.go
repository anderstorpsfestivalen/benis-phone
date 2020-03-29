package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/dtmf"

	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
)

type Controller struct {
	phone phone.Phone
	mpd   mpd.MpdClient
	dtmf  dtmf.Dtmf
}

func New(ph phone.Phone, mpd mpd.MpdClient, dtmf dtmf.Dtmf) Controller {
	return Controller{
		phone: ph,
		mpd:   mpd,
		dtmf:  dtmf,
	}
}

func (c *Controller) Start() {

	for {
		s := <-c.phone.HookChannel
		if s {
			select {
			case dtmf_key := <-c.dtmf.HookKey:
				c.MainMenu(dtmf_key)
			}
		} else {
			c.mpd.Clear()
		}
	}
}

func (c *Controller) MainMenu(dtmf_key string) {

	switch dtmf_key {
	case "1":
		message := "orvars korvar och makaroner"
		polly.TTS(message, "Astrid")
		fmt.Println(dtmf_key, message)

		c.mpd.Add("test.mp3")
		c.mpd.Play()
	case "2":
		message := "penis lasse"
		polly.TTS(message, "Astrid")
		fmt.Println(dtmf_key, message)

		c.mpd.Add("test.mp3")
		c.mpd.Play()
	}
}
