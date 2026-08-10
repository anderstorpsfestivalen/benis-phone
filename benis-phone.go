package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/anderstorpsfestivalen/benis-phone/core/api"
	"github.com/anderstorpsfestivalen/benis-phone/core/audio"
	"github.com/anderstorpsfestivalen/benis-phone/core/bridge"
	"github.com/anderstorpsfestivalen/benis-phone/core/filesync"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/hotreload"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/anderstorpsfestivalen/benis-phone/core/sip"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
)

// Version may be replaced at build time with -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	enableS3 := flag.Bool("s3", true, "sync audio files from the configured R2 bucket")
	enableHTTP := flag.Bool("http", true, "run the local HTTP recording server")
	debug := flag.Bool("debug", false, "verbose logging (DebugLevel + SIP wire tracing)")
	listAudioDevices := flag.Bool("list-audio-devices", false, "list host audio capture devices and exit")
	registrationID := flag.String("register", "", "config registration ID (required to run)")
	identityOverride := flag.String("bridge-identity", "", "override the bridge identity file path")
	resetBridge := flag.Bool("reset-bridge", false, "delete the exact bridge identity file and exit")
	remoteURL := flag.String("remote-url", "https://ivr.anderstorpsfestivalen.se", "base URL of the bridge worker")
	reloadInterval := flag.Duration("reload-interval", 60*time.Second, "hash polling interval when -poll is enabled; 0 disables")
	poll := flag.Bool("poll", false, "enable signed HTTP hash polling in addition to WebSocket updates")
	flag.Parse()

	if *listAudioDevices {
		devices, err := audio.EnumerateInputDevices()
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		for _, device := range devices {
			fmt.Printf("%-40s  channels=%d  rate=%dHz\n", device.Name, device.Channels, device.Rate)
		}
		return
	}

	log := logrus.New()
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
		log.SetLevel(logrus.DebugLevel)
		sip.EnableWireTrace()
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	identityPath, err := resolveIdentityPath(*registrationID, *identityOverride)
	if err != nil {
		log.Fatal(err)
	}
	if *resetBridge {
		if err := os.Remove(identityPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Fatalf("reset bridge identity %s: %v", identityPath, err)
		}
		log.WithField("path", identityPath).Info("Bridge identity reset")
		return
	}
	if *registrationID == "" {
		log.Fatal("-register is required")
	}

	identity, err := loadOrCreateIdentity(identityPath, *registrationID)
	if err != nil {
		log.Fatal(err)
	}
	if identity.BridgeID == "" {
		fmt.Printf("Bridge fingerprint: %s\n", bridge.Fingerprint(identity.PublicKey))
		fmt.Println("Waiting for an operator to approve this fingerprint in Registrations…")
		hostname, hostnameErr := os.Hostname()
		if hostnameErr != nil || hostname == "" {
			hostname = "unknown"
		}
		enrollmentCtx, stopEnrollment := signal.NotifyContext(
			contextBackground(),
			syscall.SIGINT,
			syscall.SIGTERM,
		)
		bridgeID, enrollmentErr := (&bridge.EnrollmentClient{
			BaseURL: *remoteURL,
			Version: Version,
		}).WaitForApproval(
			enrollmentCtx,
			identity,
			hostname,
			runtime.GOOS+"/"+runtime.GOARCH,
			2*time.Second,
		)
		stopEnrollment()
		if enrollmentErr != nil {
			log.Fatalf("bridge enrollment: %v (identity retained at %s)", enrollmentErr, identityPath)
		}
		identity.BridgeID = bridgeID
		if err := bridge.SaveIdentity(identityPath, identity); err != nil {
			log.Fatalf("save approved bridge identity: %v", err)
		}
		log.WithField("bridge_id", bridgeID).Info("Bridge enrollment approved")
	}

	signer, err := bridge.NewSigner(identity)
	if err != nil {
		log.Fatal(err)
	}
	remoteClient := functions.NewRemoteClient(*remoteURL, signer)
	initial, err := remoteClient.LoadRuntimeConfig()
	if err != nil {
		log.Fatalf("load bridge runtime from %s: %v", *remoteURL, err)
	}
	if initial.Definition.SIP.RecordPath == "" {
		initial.Definition.SIP.RecordPath = "files/recording"
	}

	defaultProvider, providers, err := prepareTTS(initial.Definition, initial.Credentials)
	if err != nil {
		log.Fatal(err)
	}
	ttsRegistry := tts.NewRegistry("haschcache", defaultProvider)
	if err := ttsRegistry.Replace(defaultProvider, providers...); err != nil {
		log.Fatal(err)
	}
	secrets.Replace(initial.Credentials)

	statusChannel := make(chan []byte, 256)
	statusReporter := func(event sip.StatusEvent) {
		payload, marshalErr := encodeSIPStatus(identity.BridgeID, event)
		if marshalErr != nil {
			return
		}
		select {
		case statusChannel <- payload:
		default:
			log.Warn("SIP status queue full; current state will be replayed after reconnect")
		}
	}
	sipSupervisor := sip.NewSupervisor(ttsRegistry, initial.Definition, statusReporter, log)
	resources := &runtimeResources{
		log:           log,
		ttsRegistry:   ttsRegistry,
		sipSupervisor: sipSupervisor,
		enableS3:      *enableS3,
	}
	if err := resources.syncR2(initial.Credentials); err != nil {
		log.Fatal(err)
	}
	if err := sipSupervisor.Start(initial.Definition, initial.SIPPasswords); err != nil {
		log.Fatal("start SIP supervisor: ", err)
	}
	log.WithFields(logrus.Fields{
		"connections": len(initial.Definition.SIP.Connections),
		"bridge_id":   identity.BridgeID,
		"hash":        hotreload.ShortHash(initial.Revision),
	}).Info("Bridge runtime started")

	statusSnapshot := func() [][]byte {
		events := sipSupervisor.StatusSnapshot()
		payloads := make([][]byte, 0, len(events))
		for _, event := range events {
			payload, marshalErr := encodeSIPStatus(identity.BridgeID, event)
			if marshalErr == nil {
				payloads = append(payloads, payload)
			}
		}
		return payloads
	}
	reloader := hotreload.New(hotreload.Config{
		RemoteClient:   remoteClient,
		SIPSupervisor:  sipSupervisor,
		ApplyRuntime:   resources.Apply,
		InstanceID:     identity.BridgeID,
		Status:         statusChannel,
		StatusSnapshot: statusSnapshot,
		InitialHash:    initial.Revision,
		Poll:           *poll,
		PollInterval:   *reloadInterval,
		Logger:         log,
	})
	reloader.Start()

	var waitGroup sync.WaitGroup
	if *enableHTTP {
		waitGroup.Add(1)
		server := api.Server{}
		go server.Start(&waitGroup)
	}

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalChannel
	log.Info("Shutting down")
	reloader.Stop()
	sipSupervisor.Stop()
}

// contextBackground is a tiny seam that keeps signal enrollment setup easy
// to exercise without storing a process-global context.
func contextBackground() context.Context { return context.Background() }

func resolveIdentityPath(registrationID, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if registrationID == "" {
		return "", fmt.Errorf("-register or -bridge-identity is required")
	}
	return bridge.DefaultIdentityPath(registrationID)
}

func loadOrCreateIdentity(path, registrationID string) (bridge.Identity, error) {
	identity, err := bridge.LoadIdentity(path)
	if err == nil {
		if !strings.EqualFold(identity.RegistrationID, registrationID) {
			return bridge.Identity{}, fmt.Errorf(
				"bridge identity %s belongs to registration %s, not %s",
				path,
				identity.RegistrationID,
				registrationID,
			)
		}
		return identity, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return bridge.Identity{}, fmt.Errorf("load bridge identity %s: %w", path, err)
	}
	identity, err = bridge.NewIdentity(registrationID)
	if err != nil {
		return bridge.Identity{}, err
	}
	if err := bridge.SaveIdentity(path, identity); err != nil {
		return bridge.Identity{}, err
	}
	return identity, nil
}

func encodeSIPStatus(instanceID string, event sip.StatusEvent) ([]byte, error) {
	return json.Marshal(struct {
		Type       string `json:"type"`
		InstanceID string `json:"instance_id"`
		sip.StatusEvent
	}{Type: "sip-status", InstanceID: instanceID, StatusEvent: event})
}

type runtimeResources struct {
	mu            sync.Mutex
	log           *logrus.Logger
	ttsRegistry   *tts.Registry
	sipSupervisor *sip.Supervisor
	enableS3      bool
	prepareTTSFn  func(functions.Definition, secrets.Credentials) (string, []tts.Provider, error)
	syncR2Fn      func(secrets.Credentials) error
	applySIPFn    func(functions.Definition, map[string]string) error
}

func (resources *runtimeResources) Apply(runtimeConfig functions.RuntimeConfig) error {
	resources.mu.Lock()
	defer resources.mu.Unlock()
	if runtimeConfig.Definition.SIP.RecordPath == "" {
		runtimeConfig.Definition.SIP.RecordPath = "files/recording"
	}
	if err := runtimeConfig.Definition.Validate(); err != nil {
		return err
	}
	prepare := prepareTTS
	if resources.prepareTTSFn != nil {
		prepare = resources.prepareTTSFn
	}
	defaultProvider, providers, err := prepare(
		runtimeConfig.Definition,
		runtimeConfig.Credentials,
	)
	if err != nil {
		return err
	}
	syncR2 := resources.syncR2
	if resources.syncR2Fn != nil {
		syncR2 = resources.syncR2Fn
	}
	if err := syncR2(runtimeConfig.Credentials); err != nil {
		return err
	}
	if err := resources.ttsRegistry.Replace(defaultProvider, providers...); err != nil {
		return err
	}
	secrets.Replace(runtimeConfig.Credentials)
	applySIP := resources.applySIPFn
	if applySIP == nil {
		if resources.sipSupervisor == nil {
			return fmt.Errorf("runtime has no SIP applier")
		}
		applySIP = resources.sipSupervisor.Apply
	}
	if err := applySIP(runtimeConfig.Definition, runtimeConfig.SIPPasswords); err != nil {
		return err
	}
	return nil
}

func (resources *runtimeResources) syncR2(credentials secrets.Credentials) error {
	if !resources.enableS3 {
		return nil
	}
	r2 := credentials.R2
	if r2.Bucket == "" {
		r2.Bucket = "ivr"
	}
	if r2.AccessKeyID == "" || r2.SecretAccessKey == "" || r2.AccountID == "" {
		return fmt.Errorf("initialize R2 sync: access key, secret key, and account ID are required")
	}
	fileSync, err := filesync.Create(filesync.Config{
		AccessKeyID:     r2.AccessKeyID,
		SecretAccessKey: r2.SecretAccessKey,
		AccountID:       r2.AccountID,
		Bucket:          r2.Bucket,
	})
	if err != nil {
		return fmt.Errorf("initialize R2 sync: %w", err)
	}
	if err := fileSync.Start("files/"); err != nil {
		return err
	}
	return nil
}

func prepareTTS(
	definition functions.Definition,
	credentials secrets.Credentials,
) (string, []tts.Provider, error) {
	defaultProvider := definition.General.DefaultTTSProvider
	if defaultProvider == "" {
		defaultProvider = "polly"
	}
	providers := make([]tts.Provider, 0, 2)
	if credentials.Polly.Key != "" || credentials.Polly.Secret != "" {
		pollyClient, err := polly.New(
			credentials.Polly.Key,
			credentials.Polly.Secret,
			"haschcache",
		)
		if err != nil {
			return "", nil, err
		}
		providers = append(providers, tts.NewPollyProvider(pollyClient))
	}
	if credentials.ElevenLabs != "" {
		providers = append(
			providers,
			tts.NewElevenLabsProvider(credentials.ElevenLabs, ""),
		)
	}
	foundDefault := false
	for _, provider := range providers {
		if provider.Name() == defaultProvider {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		return "", nil, fmt.Errorf(
			"default_tts_provider=%q is not registered (configure its credentials)",
			defaultProvider,
		)
	}
	return defaultProvider, providers, nil
}
