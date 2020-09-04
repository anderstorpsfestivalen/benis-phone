package main

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/filesync"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/secrets"
	"gitlab.com/anderstorpsfestivalen/benis-phone/virtual"
)

func main() {

	credentials := secrets.LoadSecrets()

	//Synchronize files from S3
	fsx, err := filesync.Create(credentials.S3.Key, credentials.S3.Secret, "anderstorpsfestivalen", "eu-north-1")
	if err != nil {
		log.Fatal("Could not initialize sync")
	}
	fsx.Start("files/")

	//gpioDisabled := flag.Bool("gpio", true, "blah")
	//flag.Parse()

	virtual := virtual.New()
	mpd, err := mpd.Init("127.0.0.1:6600")
	if err != nil {
		log.WithFields(log.Fields{
			"MPD error": err,
		}).Panic("Could not initiate connection to MPD")
	}

	polly := polly.New(credentials.Polly.Key, credentials.Polly.Secret, "/home/wberg/Music")

	log.Info("Starting Controller")
	log.SetLevel(log.DebugLevel)
	ctrl := controller.New(virtual, mpd, polly)

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	virtual.Init()

	go ctrl.Start(&waitgroup)

	waitgroup.Wait()

}
