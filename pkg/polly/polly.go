package polly

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	golang_tts "github.com/leprosus/golang-tts"
)

type AWS struct {
	aws_key    string
	aws_secret string
}

type Polly struct {
	credentials AWS
	haschcache  string
	fp          string
}

func New(key string, secret string, haschcache string) Polly {

	os.MkdirAll(haschcache, os.ModePerm)

	return Polly{
		credentials: AWS{
			aws_key:    key,
			aws_secret: secret,
		},
		haschcache: haschcache,
	}
}

//TTS generates a message in Swedish
func (p *Polly) TTS(message string, voice string) ([]byte, error) {

	return p.internalTTS(message, "sv-SE", voice)
}

//TTSLang allows you to define the speaking language in addition to voice.
// Yes we are quite lazy.
func (p *Polly) TTSLang(message string, language string, voice string) ([]byte, error) {

	return p.internalTTS(message, language, voice)
}

func (p *Polly) internalTTS(message string, language string, voice string) ([]byte, error) {

	cached, err := p.checkHaschCache(message, language, voice)
	if err == nil {
		return cached, nil
	}

	polly := golang_tts.New(p.credentials.aws_key, p.credentials.aws_secret)
	polly.Language(language)
	polly.Format(golang_tts.MP3)
	polly.Voice(voice)

	bytes, err := polly.Speech(message)
	if err != nil {
		return nil, err
	}

	err = p.writeHaschCache(p.haschRequest(message, language, voice), bytes)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (p *Polly) checkHaschCache(message string, language string, voice string) ([]byte, error) {

	hasch := p.haschRequest(message, language, voice)

	_, err := os.Stat(path.Join(p.haschcache, hasch))
	if os.IsNotExist(err) {
		return nil, err
	}

	return ioutil.ReadFile(path.Join(p.haschcache, hasch))
}

func (p *Polly) writeHaschCache(hasch string, data []byte) error {
	f, err := os.Create(path.Join(p.haschcache, hasch))
	_, err = f.Write(data)
	return err
}

func (p *Polly) haschRequest(message string, language string, voice string) string {
	nc := sha1.New()
	io.WriteString(nc, message+language+voice)
	return fmt.Sprintf("%x", nc.Sum(nil))
}
