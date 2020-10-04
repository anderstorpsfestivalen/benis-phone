package controller

import log "github.com/sirupsen/logrus"

type Err struct {
}

func (m *Err) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	log.Error(menu.Error)

	c.Audio.PlayFromFile("files/xperror2.wav")

	errPrefix := "Wogberg is drunk: "

	ttsData, err := c.Polly.TTSLang(errPrefix+menu.Error.Error(), "en-US", "Joanna")
	if err != nil {
		c.Audio.PlayFromFile("files/xperror2.wav")
		log.Error(err)
	}

	c.Audio.PlayMP3FromStream(ttsData)

	return MenuReturn{
		NextFunction: "mainmenu",
	}

}

func (m *Err) InputLength() int {
	return 0
}

func (m *Err) Name() string {
	return "error"
}

func (m *Err) Prefix(c *Controller) {
}
