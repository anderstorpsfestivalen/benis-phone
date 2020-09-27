package main

import (
	"flag"
	"sync"

	log "github.com/sirupsen/logrus"

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

	credentials, err := secrets.LoadSecrets()
	if err != nil {
		log.Error(err)
		panic("COULD NOT LOAD CREDENTIALS")
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

	enablePhone := flag.Bool("phone", false, "blah")
	flag.Parse()

	var ctrlPhone phone.FlowPhone

	if *enablePhone {
		phone := phone.New(5, 6, 13, 19, 26, 18)
		virtual := virtual.New()
		ctrlPhone = muxer.New(phone, virtual)
	} else {
		ctrlPhone = virtual.New()
	}

	ad := audio.New(44100)
	err = ad.Init()
	if err != nil {
		log.Fatal(err)
	}

	polly := polly.New(credentials.Polly.Key, credentials.Polly.Secret)

	log.Info("Starting Controller")
	log.SetLevel(log.DebugLevel)
	ctrl := controller.New(ctrlPhone, ad, polly)

	var waitgroup sync.WaitGroup
	waitgroup.Add(1)

	// resp, err := systemet.QueryProductNumberShort(11392)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(resp.Artikelid)

	err = ctrlPhone.Init()
	if err != nil {
		panic(err)
	}

	go ctrl.Start(&waitgroup)

	waitgroup.Wait()

}
