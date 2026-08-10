package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anderstorpsfestivalen/benis-phone/core/bridge"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/secrets"
	"github.com/anderstorpsfestivalen/benis-phone/core/tts"
)

func TestInterruptedEnrollmentIdentityIsRecovered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.json")
	registrationID := "123e4567-e89b-42d3-a456-426614174000"
	first, err := loadOrCreateIdentity(path, registrationID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loadOrCreateIdentity(path, registrationID)
	if err != nil {
		t.Fatal(err)
	}
	if first.RequestID != second.RequestID || bridge.Fingerprint(first.PublicKey) != bridge.Fingerprint(second.PublicKey) {
		t.Fatal("pending identity was replaced instead of recovered")
	}
}

func TestDiscoverSavedIdentity(t *testing.T) {
	directory := t.TempDir()
	if _, _, err := discoverSavedIdentity(filepath.Join(directory, "missing")); err == nil || !strings.Contains(err.Error(), "no saved bridge identity") {
		t.Fatalf("missing identity error = %v", err)
	}

	firstID := "123e4567-e89b-42d3-a456-426614174000"
	firstPath := filepath.Join(directory, firstID+".json")
	first, err := bridge.NewIdentity(firstID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.SaveIdentity(firstPath, first); err != nil {
		t.Fatal(err)
	}
	path, registrationID, err := discoverSavedIdentity(directory)
	if err != nil || path != firstPath || registrationID != firstID {
		t.Fatalf("single identity = path %q, registration %q, err %v", path, registrationID, err)
	}

	secondID := "123e4567-e89b-42d3-a456-426614174001"
	second, err := bridge.NewIdentity(secondID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.SaveIdentity(filepath.Join(directory, secondID+".json"), second); err != nil {
		t.Fatal(err)
	}
	if _, _, err := discoverSavedIdentity(directory); err == nil ||
		!strings.Contains(err.Error(), "multiple saved bridge identities") ||
		!strings.Contains(err.Error(), firstID) ||
		!strings.Contains(err.Error(), secondID) {
		t.Fatalf("multiple identity error = %v", err)
	}
}

func TestIdentityOverrideInfersRegistrationID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.json")
	registrationID := "123e4567-e89b-42d3-a456-426614174000"
	identity, err := bridge.NewIdentity(registrationID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.SaveIdentity(path, identity); err != nil {
		t.Fatal(err)
	}
	gotPath, gotRegistrationID, err := resolveIdentitySelection("", path)
	if err != nil || gotPath != path || gotRegistrationID != registrationID {
		t.Fatalf("override selection = path %q, registration %q, err %v", gotPath, gotRegistrationID, err)
	}
}

type namedProvider string

func (provider namedProvider) Name() string                { return string(provider) }
func (provider namedProvider) CacheKey(tts.Request) string { return "unused" }
func (provider namedProvider) Synthesize(tts.Request) ([]byte, error) {
	return []byte(provider), nil
}

func TestRuntimeApplyReplacesCredentialDependentResources(t *testing.T) {
	registry := tts.NewRegistry(t.TempDir(), "old")
	if err := registry.Replace("old", namedProvider("old")); err != nil {
		t.Fatal(err)
	}
	secrets.Replace(secrets.Credentials{MediaServer: "old"})
	definition := validRuntimeDefinition("new")
	credentials := secrets.Credentials{
		MediaServer: "new",
		R2:          secrets.R2Cred{AccessKeyID: "new-r2"},
	}
	order := make([]string, 0, 2)
	resources := &runtimeResources{
		ttsRegistry: registry,
		prepareTTSFn: func(functions.Definition, secrets.Credentials) (string, []tts.Provider, error) {
			return "new", []tts.Provider{namedProvider("new")}, nil
		},
		syncR2Fn: func(got secrets.Credentials) error {
			if got.R2.AccessKeyID != "new-r2" {
				t.Fatalf("R2 sync received %#v", got.R2)
			}
			order = append(order, "r2")
			return nil
		},
		applySIPFn: func(got functions.Definition, passwords map[string]string) error {
			order = append(order, "sip")
			if got.General.DefaultTTSProvider != "new" || passwords["inbound"] != "sip-secret" {
				t.Fatalf("SIP apply received wrong runtime bundle")
			}
			if registry.DefaultName() != "new" {
				t.Fatal("TTS registry was not replaced before SIP reconcile")
			}
			if secrets.Current().MediaServer != "new" {
				t.Fatal("service and HTTP credential store was not replaced before SIP reconcile")
			}
			return nil
		},
	}
	if err := resources.Apply(functions.RuntimeConfig{
		Definition:  definition,
		Credentials: credentials,
		SIPPasswords: map[string]string{
			"inbound": "sip-secret",
		},
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, ",") != "r2,sip" {
		t.Fatalf("apply order = %v", order)
	}
}

func TestRuntimeApplyFailureKeepsInstalledCredentialsAndProviders(t *testing.T) {
	registry := tts.NewRegistry(t.TempDir(), "old")
	if err := registry.Replace("old", namedProvider("old")); err != nil {
		t.Fatal(err)
	}
	secrets.Replace(secrets.Credentials{MediaServer: "old"})
	resources := &runtimeResources{
		ttsRegistry: registry,
		prepareTTSFn: func(functions.Definition, secrets.Credentials) (string, []tts.Provider, error) {
			return "new", []tts.Provider{namedProvider("new")}, nil
		},
		syncR2Fn: func(secrets.Credentials) error { return errors.New("sync failed") },
		applySIPFn: func(functions.Definition, map[string]string) error {
			t.Fatal("SIP apply ran after R2 preparation failed")
			return nil
		},
	}
	err := resources.Apply(functions.RuntimeConfig{
		Definition:  validRuntimeDefinition("new"),
		Credentials: secrets.Credentials{MediaServer: "new"},
	})
	if err == nil || err.Error() != "sync failed" {
		t.Fatalf("Apply error = %v", err)
	}
	if registry.DefaultName() != "old" || secrets.Current().MediaServer != "old" {
		t.Fatal("failed runtime bundle partially replaced live resources")
	}
}

func validRuntimeDefinition(defaultProvider string) functions.Definition {
	entrypoint := functions.Fn{Name: "main"}
	return functions.Definition{
		General:   functions.General{DefaultTTSProvider: defaultProvider},
		Functions: map[string]*functions.Fn{"main": &entrypoint},
		SIP: functions.SIPConfig{Connections: []functions.SIPConnection{{
			ID:           "inbound",
			Kind:         functions.SIPKindEndpoint,
			Registration: functions.SIPRegistrationInbound,
			Username:     "asterisk",
			LocalPort:    5099,
			AllowedCIDRs: []string{"127.0.0.0/8"},
			Entrypoint:   "main",
		}}},
	}
}

func TestIdentityRegistrationMismatchAndExactReset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.json")
	firstID := "123e4567-e89b-42d3-a456-426614174000"
	if _, err := loadOrCreateIdentity(path, firstID); err != nil {
		t.Fatal(err)
	}
	_, err := loadOrCreateIdentity(path, "123e4567-e89b-42d3-a456-426614174001")
	if err == nil || !strings.Contains(err.Error(), "belongs to registration") {
		t.Fatalf("mismatch error = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("identity still exists after exact reset: %v", err)
	}
}
