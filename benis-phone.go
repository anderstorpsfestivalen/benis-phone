package main

import (
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/filesync"
	"gitlab.com/anderstorpsfestivalen/benis-phone/mpd"
	"gitlab.com/anderstorpsfestivalen/benis-phone/virtual"
)

func main() {

	//Synchronize files from S3
	fsx, err := filesync.Create(os.Getenv("s3_key"), os.Getenv("s3_secret"), "anderstorpsfestivalen", "eu-north-1")
	if err != nil {
		log.Fatal("Could not initialize sync")
	}
	fsx.Start("files/")

	//gpioDisabled := flag.Bool("gpio", true, "blah")
	//flag.Parse()

	virtual := virtual.New()
	mpd := mpd.Init("127.0.0.1:6600")

	log.Info("Starting Controller")
	log.SetLevel(log.DebugLevel)
	ctrl := controller.New(virtual, mpd)

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	virtual.Init()

	go ctrl.Start(&waitgroup)

	waitgroup.Wait()

}
