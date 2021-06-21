package controller

import (
	"time"
)

type RecordMessage struct {
}

func (m *RecordMessage) Run(c *Controller, k string, menu MenuReturn) MenuReturn {

	// Clear and stop audio recording
	c.Audio.Clear()
	c.Recorder.Stop()

	time.Sleep(30 * time.Millisecond)

	c.Audio.PlayFromFile("files/record-message.ogg")

	tm := time.Now()
	recTime := tm.Format("2006-01-02_15:04:05")
	c.Recorder.Record("files/recording/message/" + recTime)

	return MenuReturn{
		NextFunction: "nil",
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
