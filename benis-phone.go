package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
	"github.com/anderstorpsfestivalen/benis-phone/core/sip"
	"github.com/anderstorpsfestivalen/benis-phone/core/virtual"
	sipgosip "github.com/emiago/sipgo/sip"
)

// SIPTracer logs SIP messages for debugging
type SIPTracer struct{}

func (t *SIPTracer) SIPTraceRead(transport, laddr, raddr string, sipmsg []byte) {
	logrus.Debugf("SIP READ [%s] %s <- %s:\n%s", transport, laddr, raddr, string(sipmsg))
}

func (t *SIPTracer) SIPTraceWrite(transport, laddr, raddr string, sipmsg []byte) {
	logrus.Debugf("SIP WRITE [%s] %s -> %s:\n%s", transport, laddr, raddr, string(sipmsg))
}

func main() {
	enableS3 := flag.Bool("s3", true, "s3 sync")
	enableHttp := flag.Bool("http", true, "http server")
	enablePhone := flag.Bool("phone", false, "Enable GPIO for physical phone")
	_ = flag.Bool("record", true, "record audio") // TODO: wire up recording toggle
	debug := flag.Bool("debug", false, "verbose logging (DebugLevel + SIP wire tracing)")
	definition := flag.String("def",
		"configurations/default.toml",
		"Set a custom definition file, standard is configurations/default.toml")
	flag.Parse()

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
		sipgosip.SIPDebug = true
		sipgosip.SIPDebugTracer(&SIPTracer{})
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

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
	rec := audio.NewRecorder("files/recording", log)
	

	// Setup Polly
	polly, err := polly.New(credentials.Polly.Key, credentials.Polly.Secret, "haschcache")
	if err != nil {
		log.Error(err)
	}

	// Setup Systemet
	// This is an ugly hack
	//systemet.InitalizeServices()

	// Load definition
	def, err := functions.LoadFromFile(*definition)
	if err != nil {
		panic(err)
	}

	var waitgroup sync.WaitGroup

	// Check if SIP mode is enabled
	if def.SIP.Enabled {
		log.Info("Starting in SIP client mode")

		// Validate required SIP config
		if def.SIP.Server == "" {
			log.Fatal("SIP server address is required when SIP is enabled")
		}
		if def.SIP.Extension == "" {
			log.Fatal("SIP extension is required when SIP is enabled")
		}

		// Configure SIP client
		sipConfig := sip.ClientConfig{
			Server:        def.SIP.Server,
			Extension:     def.SIP.Extension,
			Username:      def.SIP.Username,
			Password:      credentials.SIP.Password,
			Domain:        def.SIP.Domain,
			Transport:     def.SIP.Transport,
			LocalPort:     def.SIP.LocalPort,
			ExpirySeconds: def.SIP.ExpirySeconds,
			RecordPath:    def.SIP.RecordPath,
			ExternalIP:    def.SIP.ExternalIP,
		}

		// Set defaults
		if sipConfig.Transport == "" {
			sipConfig.Transport = "udp"
		}
		if sipConfig.RecordPath == "" {
			sipConfig.RecordPath = "files/recording"
		}

		maxCalls := def.SIP.MaxConcurrentCalls
		if maxCalls <= 0 {
			maxCalls = 10
		}

		sipClient, err := sip.NewClient(sipConfig, polly, def, maxCalls)
		if err != nil {
			log.Fatal("Failed to create SIP client: ", err)
		}

		if err := sipClient.Start(); err != nil {
			log.Fatal("Failed to start SIP client: ", err)
		}

		log.WithFields(logrus.Fields{
			"server":    sipConfig.Server,
			"extension": sipConfig.Extension,
			"max_calls": maxCalls,
		}).Info("SIP client started, registering with PBX...")

		// Handle shutdown gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		if *enableHttp {
			waitgroup.Add(1)
			// Note: API server needs to be updated to work with SIP mode
			// For now, just start it for compatibility
			srv := api.Server{}
			go srv.Start(&waitgroup, nil)
		}

		// Wait for shutdown signal
		<-sigChan
		log.Info("Shutting down SIP client...")
		sipClient.Stop()

	} else {
		// Legacy mode: local phone with speaker/mic
		log.Info("Starting in local phone mode")

		ctrl := controller.New(ctrlPhone, ad, &rec, polly, def)

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
}
