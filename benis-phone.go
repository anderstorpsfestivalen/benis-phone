package main

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/anderstorpsfestivalen/benis-phone/core/api"
	"github.com/anderstorpsfestivalen/benis-phone/core/filesync"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/anderstorpsfestivalen/benis-phone/core/sip"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
)

func main() {
	enableS3 := flag.Bool("s3", true, "s3 sync")
	enableHttp := flag.Bool("http", true, "http server")
	debug := flag.Bool("debug", false, "verbose logging (DebugLevel + SIP wire tracing)")
	direct := flag.Bool("direct", false, "SIP debug mode: skip PBX registration, accept unauthenticated INVITEs")
	directPort := flag.Int("direct-port", 0, "Override SIP local bind port when -direct is set (default: TOML local_port or 5060)")
	definition := flag.String("def",
		"configurations/default.toml",
		"Set a custom definition file, standard is configurations/default.toml")
	flag.Parse()

	log := logrus.New()
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
		log.SetLevel(logrus.DebugLevel)
		sip.EnableWireTrace()
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	credentials, err := secrets.LoadSecrets()
	if err != nil {
		log.Fatal("Could not load credentials, check creds/creds.json: ", err)
	}

	if *enableS3 {
		fsx, err := filesync.Create(credentials.S3.Key, credentials.S3.Secret, "anderstorpsfestivalen", "eu-north-1")
		if err != nil {
			log.Fatal("Could not initialize S3 sync: ", err)
		}
		fsx.Start("files/")
	}

	def, err := functions.LoadFromFile(*definition)
	if err != nil {
		log.Fatal(err)
	}

	// CLI flags override the TOML for quick debug workflows.
	if *direct {
		def.SIP.Direct = true
	}
	if *directPort > 0 {
		def.SIP.LocalPort = *directPort
	}

	if def.SIP.Transport == "" {
		def.SIP.Transport = "udp"
	}
	if def.SIP.RecordPath == "" {
		def.SIP.RecordPath = "files/recording"
	}
	if !def.SIP.Direct && def.SIP.Server == "" {
		log.Fatal("SIP server address is required (or use -direct)")
	}
	if !def.SIP.Direct && def.SIP.Extension == "" {
		log.Fatal("SIP extension is required")
	}
	maxCalls := def.SIP.MaxConcurrentCalls
	if maxCalls <= 0 {
		maxCalls = 10
	}

	ttsReg := buildTTSRegistry(log, def, credentials)

	sipClient, err := sip.NewClient(sipConfigFromDef(def, credentials), ttsReg, def, maxCalls)
	if err != nil {
		log.Fatal("Failed to create SIP client: ", err)
	}

	if err := sipClient.Start(); err != nil {
		log.Fatal("Failed to start SIP client: ", err)
	}
	log.WithFields(logrus.Fields{
		"server":    def.SIP.Server,
		"extension": def.SIP.Extension,
		"direct":    def.SIP.Direct,
		"max_calls": maxCalls,
	}).Info("SIP client started")

	var wg sync.WaitGroup
	if *enableHttp {
		wg.Add(1)
		srv := api.Server{}
		go srv.Start(&wg)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down")
	sipClient.Stop()
}

func buildTTSRegistry(log *logrus.Logger, def functions.Definition, credentials secrets.Credentials) *tts.Registry {
	pollyClient, err := polly.New(credentials.Polly.Key, credentials.Polly.Secret, "haschcache")
	if err != nil {
		log.Error(err)
	}

	defaultProvider := def.General.DefaultTTSProvider
	if defaultProvider == "" {
		defaultProvider = "polly"
	}

	reg := tts.NewRegistry("haschcache", defaultProvider)
	reg.Register(tts.NewPollyProvider(pollyClient))
	if credentials.ElevenLabs != "" {
		reg.Register(tts.NewElevenLabsProvider(credentials.ElevenLabs, ""))
		log.Info("Registered ElevenLabs TTS provider")
	}
	if !reg.Has(defaultProvider) {
		log.Fatalf("default_tts_provider=%q is not registered (missing credentials?)", defaultProvider)
	}
	return reg
}

func sipConfigFromDef(def functions.Definition, credentials secrets.Credentials) sip.ClientConfig {
	return sip.ClientConfig{
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
		Direct:        def.SIP.Direct,
	}
}
