package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
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
	instanceID := flag.String("instance-id", "", "Stable runtime identity for per-instance connection status (default: hostname)")
	sipSecretsPath := flag.String("sip-secrets", "creds/sip.json", "File-source mode: JSON object mapping SIP connection IDs to passwords")
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
		sipPasswords map[string]string
	)
	switch *source {
	case "file":
		def, err = functions.LoadFromFile(*definition)
		if err != nil {
			log.Fatal(err)
		}
		sipPasswords, err = loadSIPPasswords(*sipSecretsPath)
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("loading SIP secrets %s: %v", *sipSecretsPath, err)
		}
		if sipPasswords == nil {
			sipPasswords = make(map[string]string)
		}
	case "remote":
		if *configName == "" {
			log.Fatal("-config is required when -source=remote")
		}
		if credentials.PBXConfigToken == "" {
			log.Fatal("creds.json is missing PBXConfigToken (required for -source=remote)")
		}
		remoteClient = functions.NewRemoteClient(*remoteURL, *configName, credentials.PBXConfigToken)
		runtime, loadErr := remoteClient.LoadRuntimeConfig()
		err = loadErr
		if err != nil {
			log.Fatalf("loading remote config %q from %s: %v", *configName, *remoteURL, err)
		}
		def = runtime.Definition
		sipPasswords = runtime.SIPPasswords
		currentHash = runtime.Revision
		if currentHash == "" {
			currentHash, err = remoteClient.FetchHash()
			if err != nil {
				log.Warnf("fetching initial config hash: %v", err)
			}
		}
		log.WithFields(logrus.Fields{
			"name": *configName,
			"url":  *remoteURL,
			"hash": hotreload.ShortHash(currentHash),
		}).Info("Loaded remote config")
	default:
		log.Fatalf("invalid -source %q (want file|remote)", *source)
	}

	if def.SIP.RecordPath == "" {
		def.SIP.RecordPath = "files/recording"
	}

	ttsReg := buildTTSRegistry(log, def, credentials)
	if *instanceID == "" {
		*instanceID, err = os.Hostname()
		if err != nil || *instanceID == "" {
			*instanceID = "ivr"
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`).MatchString(*instanceID) {
		log.Fatal("-instance-id must use only letters, numbers, dot, underscore, or hyphen")
	}
	// File mode has no control-plane consumer, so it deliberately avoids an
	// outgoing queue. The supervisor still records and logs current status.
	var statusCh chan []byte
	var statusReporter func(sip.StatusEvent)
	if remoteClient != nil {
		statusCh = make(chan []byte, 256)
		statusReporter = func(event sip.StatusEvent) {
			payload, marshalErr := encodeSIPStatus(*instanceID, event)
			if marshalErr != nil {
				return
			}
			select {
			case statusCh <- payload:
			default:
				log.Warn("SIP status queue full; current state will be replayed after reconnect")
			}
		}
	}
	sipSupervisor := sip.NewSupervisor(ttsReg, def, statusReporter, log)
	if err := sipSupervisor.Start(def, sipPasswords); err != nil {
		log.Fatal("Failed to start SIP supervisor: ", err)
	}
	log.WithFields(logrus.Fields{"connections": len(def.SIP.Connections), "instance_id": *instanceID}).Info("SIP supervisor started")

	// Hot-reload — only meaningful with a remote source.
	var reloader *hotreload.Manager
	if remoteClient != nil {
		statusSnapshot := func() [][]byte {
			events := sipSupervisor.StatusSnapshot()
			payloads := make([][]byte, 0, len(events))
			for _, event := range events {
				payload, marshalErr := encodeSIPStatus(*instanceID, event)
				if marshalErr == nil {
					payloads = append(payloads, payload)
				}
			}
			return payloads
		}
		reloader = hotreload.New(hotreload.Config{
			RemoteClient:   remoteClient,
			SIPSupervisor:  sipSupervisor,
			SyncFiles:      resync,
			BaseURL:        *remoteURL,
			Name:           *configName,
			Token:          credentials.PBXConfigToken,
			InstanceID:     *instanceID,
			Status:         statusCh,
			StatusSnapshot: statusSnapshot,
			InitialHash:    currentHash,
			Poll:           *poll,
			PollInterval:   *reloadInterval,
			Logger:         log,
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
	sipSupervisor.Stop()
}

func encodeSIPStatus(instanceID string, event sip.StatusEvent) ([]byte, error) {
	return json.Marshal(struct {
		Type       string `json:"type"`
		InstanceID string `json:"instance_id"`
		sip.StatusEvent
	}{Type: "sip-status", InstanceID: instanceID, StatusEvent: event})
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

func loadSIPPasswords(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var passwords map[string]string
	if err := json.Unmarshal(data, &passwords); err != nil {
		return nil, err
	}
	return passwords, nil
}
