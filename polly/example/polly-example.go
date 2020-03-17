package main

import (
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
)

func main() {

	message := "hej hej, testing testing"
	polly.TTS(message, "Emma")

}
