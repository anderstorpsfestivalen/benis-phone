package main

import (
	"io/ioutil"

	golang_tts "github.com/leprosus/golang-tts"
)

func main() {
	polly := golang_tts.New("aws_key", "aws_secret")

	polly.Format(golang_tts.MP3)
	polly.Voice(golang_tts.Astrid)

	text := "Hej hej, jag provar Astrid på Svenska"
	bytes, err := polly.Speech(text)
	if err != nil {
		panic(err)
	}

	ioutil.WriteFile("./output.mp3", bytes, 0644)
}
