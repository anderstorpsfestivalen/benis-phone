package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/virtual"
)

func main() {

	//gpioDisabled := flag.Bool("gpio", true, "blah")
	//flag.Parse()

	phone := virtual.New()
	mpd := mpd.Init("127.0.0.1:6600")

	fmt.Println("Starting controller")

	ctrl := controller.New(phone, mpd)

	go ctrl.Start()

	phone.Init(0)

}
