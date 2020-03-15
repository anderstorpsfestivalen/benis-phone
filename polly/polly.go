package polly

import (
	"io/ioutil"
	"os"
	"path"

	golang_tts "github.com/leprosus/golang-tts"
)

type AWS struct {
	aws_key    string
	aws_secret string
}

func TTS(message string, voice string) string {
	var a AWS

	// Read temp env variable for key
	a.aws_key = os.Getenv("aws_key")
	a.aws_secret = os.Getenv("aws_secret")

	polly := golang_tts.New(a.aws_key, a.aws_secret)

	polly.Format(golang_tts.MP3)
	polly.Voice(golang_tts.Astrid)

	bytes, err := polly.Speech(message)
	if err != nil {
		panic(err)
	}

	home, err := os.UserHomeDir()
	filename := path.Join(home + "/Music/test.mp3")
	ioutil.WriteFile(filename, bytes, 0644)

	return filename
}
