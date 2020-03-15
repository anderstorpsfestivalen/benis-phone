package main

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
)

func main() {
	host := "[::1]:6600"
	m := mpd.Init(host)

	defer m.Close()
	fmt.Println(m)

	m.Add("output.mp3")
	m.Add("pip.wav")
	m.Add("astrid.mp3")
	m.Add("pip.wav")
	m.Play()
	//m.Clear()
}
