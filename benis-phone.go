package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/anderstorpsfestivalen/benis-phone/core/api"
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/filesync"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/hotreload"
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
	listAudioDevices := flag.Bool("list-audio-devices", false, "List host audio capture devices (for livefeed config) and exit")
	definition := flag.String("def",
		"configurations/default.toml",
		"Path to TOML config when -source=file")
	source := flag.String("source", "remote", "Config source: remote | file")
	configName := flag.String("config", "", "Remote config name (required when -source=remote)")
	flag.StringVar(configName, "c", "", "Alias for -config")
	remoteURL := flag.String("remote-url",
		"https://ivr.anderstorpsfestivalen.se",
		"Base URL of the config worker (used when -source=remote)")
	reloadInterval := flag.Duration("reload-interval", 60*time.Second,
		"Remote-mode: poll for config hash changes at this interval (only used with -poll). 0 disables.")
	poll := flag.Bool("poll", false,
		"Remote-mode: enable HTTP poll fallback. By default the binary subscribes to the broker WebSocket; use -poll only when WS is blocked.")
	flag.Parse()

	if *listAudioDevices {
		devs, err := audio.EnumerateInputDevices()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		for _, d := range devs {
			fmt.Printf("%-40s  channels=%d  rate=%dHz\n", d.Name, d.Channels, d.Rate)
		}
		os.Exit(0)
	}

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

	// resync, when non-nil, walks the R2 bucket and pulls any keys not
	// yet on disk. It's called once at startup and again on every
	// config-update event from the WS broker, so newly-referenced files
	// land before the IVR swaps to the new Definition.
	var resync func()
	if *enableS3 {
		r2 := credentials.R2
		if r2.Bucket == "" {
			r2.Bucket = "ivr"
		}
		fsx, err := filesync.Create(filesync.Config{
			AccessKeyID:     r2.AccessKeyID,
			SecretAccessKey: r2.SecretAccessKey,
			AccountID:       r2.AccountID,
			Bucket:          r2.Bucket,
		})
		if err != nil {
			log.Fatal("Could not initialize R2 sync: ", err)
		}
		fsx.Start("files/")
		resync = func() { fsx.Start("files/") }
	}

	var (
		def          functions.Definition
		remoteClient *functions.RemoteClient
		currentHash  string
	)
	switch *source {
	case "file":
		def, err = functions.LoadFromFile(*definition)
		if err != nil {
			log.Fatal(err)
		}
	case "remote":
		if *configName == "" {
			log.Fatal("-config is required when -source=remote")
		}
		if credentials.PBXConfigToken == "" {
			log.Fatal("creds.json is missing PBXConfigToken (required for -source=remote)")
		}
		remoteClient = functions.NewRemoteClient(*remoteURL, *configName, credentials.PBXConfigToken)
		def, err = remoteClient.LoadDefinition()
		if err != nil {
			log.Fatalf("loading remote config %q from %s: %v", *configName, *remoteURL, err)
		}
		currentHash, err = remoteClient.FetchHash()
		if err != nil {
			// Non-fatal: definition already loaded; the WS push or
			// the next poll tick will retry.
			log.Warnf("fetching initial config hash: %v", err)
		}
		log.WithFields(logrus.Fields{
			"name": *configName,
			"url":  *remoteURL,
			"hash": hotreload.ShortHash(currentHash),
		}).Info("Loaded remote config")
	default:
		log.Fatalf("invalid -source %q (want file|remote)", *source)
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

	// Hot-reload — only meaningful with a remote source.
	var reloader *hotreload.Manager
	if remoteClient != nil {
		reloader = hotreload.New(hotreload.Config{
			RemoteClient: remoteClient,
			SIPClient:    sipClient,
			SyncFiles:    resync,
			BaseURL:      *remoteURL,
			Name:         *configName,
			Token:        credentials.PBXConfigToken,
			InitialHash:  currentHash,
			Poll:         *poll,
			PollInterval: *reloadInterval,
			Logger:       log,
		})
		reloader.Start()
	}

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
	if reloader != nil {
		reloader.Stop()
	}
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
