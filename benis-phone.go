package main

import (
	"flag"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"gitlab.com/anderstorpsfestivalen/benis-phone/controller"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/audio"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/filesync"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/muxer"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/phone"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/polly"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/secrets"
	"gitlab.com/anderstorpsfestivalen/benis-phone/pkg/virtual"
	"gitlab.com/anderstorpsfestivalen/benis-phone/services/systemet"
)

func main() {

	log := logrus.New()

	credentials, err := secrets.LoadSecrets()
	if err != nil {
		log.Error(err)
		panic("Could not load credentials, check creds/creds.json")
	}

	//Synchronize files from S3
	fsx, err := filesync.Create(credentials.S3.Key, credentials.S3.Secret, "anderstorpsfestivalen", "eu-north-1")
	if err != nil {
		log.Fatal("Could not initialize sync")
	}
	fsx.Start("files/")
	err = systemet.Init()
	if err != nil {
		log.Error(err)
		log.Fatal("Could not init systembolaget lookup")
	}

	// Setup GPIO if -phone is used
	enablePhone := flag.Bool("phone", false, "Enable GPIO for physical phone")
	flag.Parse()

	var ctrlPhone phone.FlowPhone

	if *enablePhone {
		phone := phone.New(5, 6, 12, 13, 16, 23)
		virtual := virtual.New()
		ctrlPhone = muxer.New(phone, virtual)
	} else {
		ctrlPhone = virtual.New()
	}

	// Some audio shit
	ad := audio.New(44100)
	err = ad.Init()
	if err != nil {
		log.Fatal(err)
	}

	// Set recording device, usually a second sound card if using a RPI
	rec := audio.NewRecorder("hw:0,0", "files/recording", log)

	// Setup Polly
	polly, err := polly.New(credentials.Polly.Key, credentials.Polly.Secret, "haschcache")
	if err != nil {
		log.Error(err)
	}

	// Setup Systemet
	key, err := systemet.GetKey()
	if err != nil {
		log.Error(err)
		log.Panic("Could not get systembolaget key")
	}

	systemetAPI := systemet.New(key)

	go func() {
		// Setup http server
		r := gin.Default()
		authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
			"recording": "penis",
		}))

		authorized.StaticFS("files/recording", http.Dir("recordings"))
		r.Run()
	}()

	// Start controller
	log.Info("Starting Controller")
	log.SetLevel(logrus.DebugLevel)
	ctrl := controller.New(ctrlPhone, ad, rec, polly, *systemetAPI, controller.ControllerSettings{
		HiddenPlayback: true,
	})

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	err = ctrlPhone.Init()
	if err != nil {
		log.Panic(err)
	}

	go ctrl.Start(&waitgroup)

	waitgroup.Wait()

}
