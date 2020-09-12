package main

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"gitlab.com/anderstorpsfestivalen/benis-phone/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/filesync"
	"gitlab.com/anderstorpsfestivalen/benis-phone/muxer"
	"gitlab.com/anderstorpsfestivalen/benis-phone/phone"
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

	phone := phone.New(5, 6, 13, 19, 26, 18)
	virtual := virtual.New()

	muxed := muxer.New(phone, virtual)

	ad := audio.New(44100)
	err = ad.Init()
	if err != nil {
		log.Fatal(err)
	}

	polly := polly.New(credentials.Polly.Key, credentials.Polly.Secret)

	log.Info("Starting Controller")
	log.SetLevel(log.DebugLevel)
	ctrl := controller.New(muxed, ad, polly)

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	err = muxed.Init()
	if err != nil {
		panic(err)
	}

	go ctrl.Start(&waitgroup)

	waitgroup.Wait()

}
