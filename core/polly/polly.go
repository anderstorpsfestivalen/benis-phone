package polly

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	log "github.com/sirupsen/logrus"

	golang_tts "github.com/anderstorpsfestivalen/go-tts"
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

var Neural = map[string]bool{
	"Arlet":     true,
	"Olivia":    true,
	"Amy":       true,
	"Emma":      true,
	"Brian":     true,
	"Arthur":    true,
	"Aria":      true,
	"Ayanda":    true,
	"Ivy":       true,
	"Joanna":    true,
	"Kendra":    true,
	"Kimberly":  true,
	"Salli":     true,
	"Joey":      true,
	"Justin":    true,
	"Kevin":     true,
	"Matthew":   true,
	"Léa":       true,
	"Gabrielle": true,
	"Liam":      true,
	"Vicki":     true,
	"Daniel":    true,
	"Hannah":    true,
	"Bianca":    true,
	"Takumi":    true,
	"Seoyeon":   true,
	"Camila":    true,
	"Vitoria":   true,
	"Ines":      true,
	"Lucia":     true,
	"Mia":       true,
	"Lupe":      true,
	"Pedro":     true,
}

func New(key string, secret string, haschcache string) (Polly, error) {

	os.MkdirAll(haschcache, os.ModePerm)

	// Error check for missing credentials in creds.json
	if key == "" || secret == "" {
		return Polly{}, fmt.Errorf("No credentials for Polly found.")
	}

	return Polly{
		credentials: AWS{
			aws_key:    key,
			aws_secret: secret,
		},
		haschcache: haschcache,
	}, nil
}

//TTS generates a message in Swedish
func (p *Polly) TTS(message string, voice string) ([]byte, error) {

	return p.internalTTS(message, "sv-SE", voice, "standard")
}

//TTSLang allows you to define the speaking language in addition to voice.
// Yes we are quite lazy.
func (p *Polly) TTSLang(message string, language string, voice string, engine string) ([]byte, error) {

	return p.internalTTS(message, language, voice, engine)
}

func (p *Polly) internalTTS(message string, language string, voice string, engine string) ([]byte, error) {

	cached, err := p.checkHaschCache(message, language, voice, engine)
	if err == nil {
		log.Trace("Haschcache hit, returning cached")
		return cached, nil
	}

	log.Trace("Haschcache miss, requesting from Polly")

	e := golang_tts.STANDARD

	if engine == "neural" && p.isNeural(voice) {
		e = golang_tts.NEURAL
	}

	polly := golang_tts.New(p.credentials.aws_key, p.credentials.aws_secret)
	polly.Language(language)
	polly.Engine(e)
	polly.Format(golang_tts.MP3)
	polly.Voice(voice)

	bytes, err := polly.Speech(message)
	if err != nil {
		return nil, err
	}

	err = p.writeHaschCache(p.haschRequest(message, language, voice, engine), bytes)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (p *Polly) isNeural(voice string) bool {
	if _, ok := Neural[voice]; ok {
		return true
	}
	return false
}

func (p *Polly) checkHaschCache(message string, language string, voice string, engine string) ([]byte, error) {

	hasch := p.haschRequest(message, language, voice, engine)

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

func (p *Polly) haschRequest(message string, language string, voice string, engine string) string {
	nc := sha1.New()
	io.WriteString(nc, message+language+voice+engine)
	return fmt.Sprintf("%x", nc.Sum(nil))
}
