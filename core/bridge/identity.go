// Package bridge owns the local Ed25519 identity used to enroll and
// authenticate a benis-phone runtime.
package bridge

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const IdentityVersion = 1

type Identity struct {
	Version        int    `json:"version"`
	RegistrationID string `json:"registration_id"`
	RequestID      string `json:"request_id"`
	PublicKey      []byte `json:"public_key"`
	PrivateKey     []byte `json:"private_key"`
	BridgeID       string `json:"bridge_id,omitempty"`
}

func NewIdentity(registrationID string) (Identity, error) {
	if _, err := uuid.Parse(registrationID); err != nil {
		return Identity{}, fmt.Errorf("invalid registration id: %w", err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, fmt.Errorf("generate Ed25519 identity: %w", err)
	}
	return Identity{
		Version:        IdentityVersion,
		RegistrationID: registrationID,
		RequestID:      uuid.NewString(),
		PublicKey:      append([]byte(nil), publicKey...),
		PrivateKey:     append([]byte(nil), privateKey...),
	}, nil
}

func (i Identity) Validate() error {
	if i.Version != IdentityVersion {
		return fmt.Errorf("unsupported bridge identity version %d", i.Version)
	}
	if _, err := uuid.Parse(i.RegistrationID); err != nil {
		return fmt.Errorf("invalid saved registration id: %w", err)
	}
	if _, err := uuid.Parse(i.RequestID); err != nil {
		return fmt.Errorf("invalid saved request id: %w", err)
	}
	if len(i.PublicKey) != ed25519.PublicKeySize || len(i.PrivateKey) != ed25519.PrivateKeySize {
		return errors.New("saved bridge identity has invalid Ed25519 key lengths")
	}
	derived := ed25519.PrivateKey(i.PrivateKey).Public().(ed25519.PublicKey)
	if !derived.Equal(ed25519.PublicKey(i.PublicKey)) {
		return errors.New("saved bridge public and private keys do not match")
	}
	if i.BridgeID != "" {
		if _, err := uuid.Parse(i.BridgeID); err != nil {
			return fmt.Errorf("invalid saved bridge id: %w", err)
		}
	}
	return nil
}

func DefaultIdentityPath(registrationID string) (string, error) {
	if _, err := uuid.Parse(registrationID); err != nil {
		return "", fmt.Errorf("invalid registration id: %w", err)
	}
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve OS user config directory: %w", err)
	}
	return filepath.Join(root, "benis-phone", "bridges", strings.ToLower(registrationID)+".json"), nil
}

func LoadIdentity(path string) (Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Identity{}, err
	}
	var identity Identity
	if err := json.Unmarshal(data, &identity); err != nil {
		return Identity{}, fmt.Errorf("decode bridge identity: %w", err)
	}
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// SaveIdentity atomically replaces path. The containing directory and final
// identity file are tightened even when they already existed.
func SaveIdentity(path string, identity Identity) error {
	if err := identity.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	_, statErr := os.Stat(dir)
	createdDirectory := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !createdDirectory {
		return fmt.Errorf("inspect bridge identity directory: %w", statErr)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create bridge identity directory: %w", err)
	}
	if createdDirectory {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("secure bridge identity directory: %w", err)
		}
	}
	temporary, err := os.CreateTemp(dir, ".bridge-identity-*")
	if err != nil {
		return fmt.Errorf("create temporary bridge identity: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}
	if err := temporary.Chmod(0o600); err != nil {
		cleanup()
		return fmt.Errorf("secure temporary bridge identity: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(identity); err != nil {
		cleanup()
		return fmt.Errorf("write bridge identity: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync bridge identity: %w", err)
	}
	if err := temporary.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close bridge identity: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		cleanup()
		return fmt.Errorf("install bridge identity: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure bridge identity: %w", err)
	}
	return nil
}

func Fingerprint(publicKey []byte) string {
	digest := sha256.Sum256(publicKey)
	hexDigest := hex.EncodeToString(digest[:])
	parts := make([]string, 0, len(hexDigest)/2)
	for len(hexDigest) >= 2 {
		parts = append(parts, hexDigest[:2])
		hexDigest = hexDigest[2:]
	}
	return strings.Join(parts, ":")
}
