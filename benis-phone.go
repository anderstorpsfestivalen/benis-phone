package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/dtmf"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
)

func main() {

	dtmf := dtmf.Init()
	ph := phone.Init(6)
	mpd := mpd.Init("127.0.0.1:6600")

	ctrl := controller.New(ph, mpd)

	fmt.Println(ctrl)
	//ctrl.Start()

	dtmf_key := <-dtmf.HookKey

	switch dtmf_key {
	case "1":
		message := "orvars korvar och makaroner"
		polly.TTS(message, "Astrid")
		fmt.Println(dtmf_key, message)

		mpd.Add("test.mp3")
		mpd.Play()
	case "2":
		message := "penis lasse"
		polly.TTS(message, "Astrid")
		fmt.Println(dtmf_key, message)

		mpd.Add("test.mp3")
		mpd.Play()
	}

}
