package polly

import (
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

func New(key string, secret string) Polly {

	return Polly{
		credentials: AWS{
			aws_key:    key,
			aws_secret: secret,
		},
	}
}

func (p *Polly) TTS(message string, voice string) ([]byte, error) {

	polly := golang_tts.New(p.credentials.aws_key, p.credentials.aws_secret)
	polly.Language("sv-SE")
	polly.Format(golang_tts.MP3)
	polly.Voice(voice)

	bytes, err := polly.Speech(message)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}
