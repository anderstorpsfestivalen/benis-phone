package controller

import (
	"math/rand"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
)

func (c *Controller) handleQueue(q functions.Queue) {

	rand.Seed(time.Now().UTC().UnixNano())

	// play entry message
	q.EntryMessage.Play(c.Audio, c.Polly)

	// start background audio
	q.StartBackground(c.Audio)
}
