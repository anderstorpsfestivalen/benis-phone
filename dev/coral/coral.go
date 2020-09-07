package main

import "gitlab.com/anderstorpsfestivalen/benis-phone/audio"

func main() {
	ad := audio.New(44100)
	ad.Init()
}
