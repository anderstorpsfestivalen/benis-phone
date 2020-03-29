package main

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/dtmf"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
)

func main() {

	dtmf := dtmf.Init()
	ph := phone.Init(6)
	mpd := mpd.Init("127.0.0.1:6600")

	ctrl := controller.New(ph, mpd, dtmf)

	ctrl.Start()

}
