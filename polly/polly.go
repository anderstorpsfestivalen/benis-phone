package polly

import (
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/google/uuid"
	golang_tts "github.com/leprosus/golang-tts"
)

type AWS struct {
	aws_key    string
	aws_secret string
}

type Polly struct {
	credentials AWS
	fp          string
}

func New(key string, secret string, filepath string) Polly {

	return Polly{
		credentials: AWS{
			aws_key:    key,
			aws_secret: secret,
		},
		fp: filepath,
	}
}

func (p *Polly) TTS(message string, voice string) (string, error) {

	polly := golang_tts.New(p.credentials.aws_key, p.credentials.aws_secret)
	polly.Language("sv-SE")
	polly.Format(golang_tts.MP3)
	polly.Voice(voice)

	bytes, err := polly.Speech(message)
	if err != nil {
		return "", err
	}

	filename := path.Join(p.fp + "/" + uuid.New().String() + ".mp3")
	ioutil.WriteFile(filename, bytes, 0644)

	return filepath.Base(filename), nil
}
