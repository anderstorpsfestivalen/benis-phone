package main

import (
	"flag"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/anderstorpsfestivalen/benis-phone/core/api"
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/controller"
	"github.com/anderstorpsfestivalen/benis-phone/core/filesync"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/muxer"
	"github.com/anderstorpsfestivalen/benis-phone/core/phone"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/anderstorpsfestivalen/benis-phone/core/virtual"
	"github.com/anderstorpsfestivalen/benis-phone/extensions/services/systemet"
)

func main() {
	enableS3 := flag.Bool("s3", true, "s3 sync")
	enableHttp := flag.Bool("http", true, "http server")
	enablePhone := flag.Bool("phone", false, "Enable GPIO for physical phone")
	definition := flag.String("def",
		"configurations/default.toml",
		"Set a custom definition file, standard is configurations/default.toml")
	flag.Parse()

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

	if *enableS3 {
		fsx.Start("files/")
	}

	// Setup GPIO if -phone is used
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
	// This is an ugly hack
	systemet.InitalizeServices()

	// Load definition
	def, err := functions.LoadFromFile(*definition)
	if err != nil {
		panic(err)
	}

	// Start controller
	log.Info("Starting Controller")
	log.SetLevel(logrus.DebugLevel)
	ctrl := controller.New(ctrlPhone, ad, rec, polly, def)

	var waitgroup sync.WaitGroup

	//Phone
	waitgroup.Add(1)
	err = ctrlPhone.Init()
	if err != nil {
		log.Panic(err)
	}

	go ctrl.Start(&waitgroup)

	if *enableHttp {
		waitgroup.Add(1)
		srv := api.Server{}
		srv.Start(&waitgroup, &ctrl)
	}

	waitgroup.Wait()

}
