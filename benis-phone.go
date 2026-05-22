package main

import (
	"context"
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
	wsCtx, wsCancel := context.WithCancel(context.Background())
	defer wsCancel()
	if remoteClient != nil {
		// Default: long-lived WS subscription to the broker. The OnUpdate
		// callback shares the same reload path as polling / SIGUSR1.
		var hashMu sync.Mutex
		watcher := &functions.WSWatcher{
			BaseURL: *remoteURL,
			Name:    *configName,
			Token:   credentials.PBXConfigToken,
			Logger:  log,
			OnUpdate: func(hash string) {
				hashMu.Lock()
				defer hashMu.Unlock()
				// reloadOnce re-syncs R2 (when enabled) before swapping
				// the Definition, so any newly-referenced audio files land
				// on disk before calls pick up the new menu.
				reloadOnce(log, remoteClient, sipClient, &currentHash, false, resync)
			},
		}
		reloadWg.Add(1)
		go func() {
			defer reloadWg.Done()
			watcher.Run(wsCtx)
		}()

		// Optional poll fallback for environments where the WS upgrade is
		// blocked (corporate proxies, certain captive networks).
		if *poll && *reloadInterval > 0 {
			reloadWg.Add(1)
			go func() {
				defer reloadWg.Done()
				runHotReload(log, remoteClient, sipClient, &currentHash, *reloadInterval, stopReload, resync)
			}()
		}

		// SIGUSR1 forces an immediate reload — useful for ops and for the
		// editor to push changes faster than the next poll.
		usr1 := make(chan os.Signal, 1)
		signal.Notify(usr1, syscall.SIGUSR1)
		go func() {
			for range usr1 {
				log.Info("SIGUSR1 received, forcing config reload")
				reloadOnce(log, remoteClient, sipClient, &currentHash, true, resync)
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
	wsCancel()
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
	syncFiles func(),
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
			reloadOnce(log, rc, sipClient, current, false, syncFiles)
		}
	}
}

// reloadOnce pulls the worker's current hash; if it differs from `current`
// (or force is set), it re-syncs the R2 bucket so any newly-referenced
// audio files land on disk before the new Definition is swapped in.
// syncFiles may be nil — typically when -s3=false.
func reloadOnce(
	log *logrus.Logger,
	rc *functions.RemoteClient,
	sipClient *sip.Client,
	current *string,
	force bool,
	syncFiles func(),
) {
	h, err := rc.FetchHash()
	if err != nil {
		log.Warnf("hot-reload: hash fetch failed: %v", err)
		return
	}
	if !force && h == *current {
		return
	}
	if syncFiles != nil {
		syncFiles()
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
