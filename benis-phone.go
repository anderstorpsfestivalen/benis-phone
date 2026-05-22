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
	source := flag.String("source", "file", "Config source: file | remote")
	configName := flag.String("config-name", "", "Remote config name (required when -source=remote)")
	remoteURL := flag.String("remote-url",
		"https://ivr.anderstorpsfestivalen.se",
		"Base URL of the config worker (used when -source=remote)")
	reloadInterval := flag.Duration("reload-interval", 60*time.Second,
		"Remote-mode: poll for config hash changes at this interval. 0 disables polling.")
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

	if *enableS3 {
		fsx, err := filesync.Create(credentials.S3.Key, credentials.S3.Secret, "anderstorpsfestivalen", "eu-north-1")
		if err != nil {
			log.Fatal("Could not initialize S3 sync: ", err)
		}
		fsx.Start("files/")
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
			log.Fatal("-config-name is required when -source=remote")
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
			// Non-fatal: definition already loaded; polling will retry.
			log.Warnf("fetching initial config hash: %v", err)
		}
		log.WithFields(logrus.Fields{
			"name": *configName,
			"url":  *remoteURL,
			"hash": shortHash(currentHash),
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

	// Hot-reload: only meaningful with a remote source.
	stopReload := make(chan struct{})
	var reloadWg sync.WaitGroup
	if remoteClient != nil && *reloadInterval > 0 {
		reloadWg.Add(1)
		go func() {
			defer reloadWg.Done()
			runHotReload(log, remoteClient, sipClient, &currentHash, *reloadInterval, stopReload)
		}()
		// SIGUSR1 forces an immediate reload — useful for ops and for the
		// editor to push changes faster than the next poll.
		usr1 := make(chan os.Signal, 1)
		signal.Notify(usr1, syscall.SIGUSR1)
		go func() {
			for range usr1 {
				log.Info("SIGUSR1 received, forcing config reload")
				reloadOnce(log, remoteClient, sipClient, &currentHash, true)
			}
		}()
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
	close(stopReload)
	reloadWg.Wait()
	sipClient.Stop()
}

// runHotReload polls the worker for hash changes and swaps the active
// Definition when one is detected. New calls pick up the new config; in-
// flight calls keep their snapshot until they end.
func runHotReload(
	log *logrus.Logger,
	rc *functions.RemoteClient,
	sipClient *sip.Client,
	current *string,
	interval time.Duration,
	stop <-chan struct{},
) {
	t := time.NewTicker(interval)
	defer t.Stop()
	log.WithField("interval", interval.String()).Info("Hot-reload loop started")
	for {
		select {
		case <-stop:
			log.Info("Hot-reload loop stopped")
			return
		case <-t.C:
			reloadOnce(log, rc, sipClient, current, false)
		}
	}
}

func reloadOnce(
	log *logrus.Logger,
	rc *functions.RemoteClient,
	sipClient *sip.Client,
	current *string,
	force bool,
) {
	h, err := rc.FetchHash()
	if err != nil {
		log.Warnf("hot-reload: hash fetch failed: %v", err)
		return
	}
	if !force && h == *current {
		return
	}
	def, err := rc.LoadDefinition()
	if err != nil {
		log.Warnf("hot-reload: definition fetch failed: %v", err)
		return
	}
	sipClient.SessionManager().UpdateDefinition(def)
	*current = h
	log.WithField("hash", shortHash(h)).Info("Hot-reloaded config")
}

func shortHash(h string) string {
	if len(h) < 8 {
		return h
	}
	return h[:8]
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
