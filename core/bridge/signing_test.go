package bridge

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSigningGoldenVector(t *testing.T) {
	var vector struct {
		Seed        string `json:"seed"`
		PublicKey   string `json:"public_key"`
		Method      string `json:"method"`
		EscapedPath string `json:"escaped_path"`
		BodyHash    string `json:"body_hash"`
		Timestamp   string `json:"timestamp"`
		Nonce       string `json:"nonce"`
		Canonical   string `json:"canonical"`
		Signature   string `json:"signature"`
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bridge-signing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &vector); err != nil {
		t.Fatal(err)
	}
	seed, err := base64.StdEncoding.DecodeString(vector.Seed)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	gotPublic := base64.StdEncoding.EncodeToString(publicKey)
	if gotPublic != vector.PublicKey {
		t.Fatalf("public_key = %q", gotPublic)
	}
	canonical := CanonicalRequest(
		vector.Method,
		vector.EscapedPath,
		vector.BodyHash,
		vector.Timestamp,
		vector.Nonce,
	)
	if canonical != vector.Canonical {
		t.Fatalf("canonical = %q", canonical)
	}
	gotSignature := base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(canonical)))
	if gotSignature != vector.Signature {
		t.Fatalf("signature = %q", gotSignature)
	}
}
