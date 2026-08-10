package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIdentitySaveIsAtomicAndPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "bridge.json")
	identity, err := NewIdentity("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIdentity(path, identity); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.RequestID != identity.RequestID || Fingerprint(loaded.PublicKey) != Fingerprint(identity.PublicKey) {
		t.Fatal("saved identity did not round trip")
	}
	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("identity directory mode = %o, want 700", got)
		}
		if got := fileInfo.Mode().Perm(); got != 0o600 {
			t.Fatalf("identity file mode = %o, want 600", got)
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".bridge-identity-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary identity files remain: %v", matches)
	}
}

func TestLoadIdentityRejectsCorruptKeyPair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge.json")
	identity, err := NewIdentity("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatal(err)
	}
	identity.PublicKey[0] ^= 0xff
	if err := SaveIdentity(path, identity); err == nil {
		t.Fatal("SaveIdentity accepted a mismatched key pair")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid identity was written: %v", err)
	}
}

func TestIdentityOverrideDoesNotChmodExistingParent(t *testing.T) {
	parent := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(parent, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(parent, "bridge.json")
	identity, err := NewIdentity("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIdentity(path, identity); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(parent)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o755 {
			t.Fatalf("existing override parent mode changed to %o", got)
		}
	}
}
