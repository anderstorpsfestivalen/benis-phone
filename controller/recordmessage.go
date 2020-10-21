package controller

import (
	"fmt"
	"time"
)

type RecordMessage struct {
}

func (m *RecordMessage) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	// Clear and stop audio recording
	c.Audio.Clear()
	c.Recorder.Stop()

	message := "Spela in ett meddelande efter pipet. För att avsluta, lägg på luren. PIIIIIP" //fix later
	//fmt.Println(message)

	ttsData, err := c.Polly.TTS(message, "Astrid")
	if err != nil {
		return MenuReturn{
			Error:        err,
			NextFunction: "error",
		}
	}
	c.Audio.PlayMP3FromStream(ttsData)

	tm := time.Now()
	recTime := tm.Format("2006-01-02_15:04:05")
	c.Recorder.Record("message/" + recTime)

	//hookstate := c.Phone.State()
	hookchan := c.Phone.GetHookChannel()

	for {
		select {
		case hook := <-hookchan:
			if hook == false {
				fmt.Println("in hook, it's slammed")
				c.Audio.Clear()
				c.Recorder.Stop()
				return MenuReturn{
					NextFunction: menu.Caller,
				}
			}
		}
	}
}

func (m *RecordMessage) InputLength() int {
	return 0
}

func (m *RecordMessage) Name() string {
	return "recordmessage"
}

func (m *RecordMessage) Prefix(c *Controller) {
}
