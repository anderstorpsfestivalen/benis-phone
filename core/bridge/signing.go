package bridge

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const SignatureVersion = "v1"

func CanonicalRequest(method, escapedPath, bodyHash, timestamp, nonce string) string {
	return strings.Join([]string{
		SignatureVersion,
		strings.ToUpper(method),
		escapedPath,
		strings.ToLower(bodyHash),
		timestamp,
		nonce,
	}, "\n")
}

type Signer struct {
	BridgeID   string
	PrivateKey ed25519.PrivateKey
	Now        func() time.Time
	Nonce      func() (string, error)
}

func NewSigner(identity Identity) (*Signer, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	if identity.BridgeID == "" {
		return nil, fmt.Errorf("bridge identity has not been approved")
	}
	return &Signer{
		BridgeID:   identity.BridgeID,
		PrivateKey: append(ed25519.PrivateKey(nil), identity.PrivateKey...),
	}, nil
}

func (s *Signer) Sign(req *http.Request, body []byte) error {
	if s == nil || len(s.PrivateKey) != ed25519.PrivateKeySize || s.BridgeID == "" {
		return fmt.Errorf("invalid bridge signer")
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	nonceFn := randomNonce
	if s.Nonce != nil {
		nonceFn = s.Nonce
	}
	nonce, err := nonceFn()
	if err != nil {
		return fmt.Errorf("generate bridge nonce: %w", err)
	}
	timestamp := strconv.FormatInt(now().Unix(), 10)
	digest := sha256.Sum256(body)
	canonical := CanonicalRequest(
		req.Method,
		req.URL.EscapedPath(),
		hex.EncodeToString(digest[:]),
		timestamp,
		nonce,
	)
	signature := ed25519.Sign(s.PrivateKey, []byte(canonical))
	req.Header.Set("X-Bridge-ID", s.BridgeID)
	req.Header.Set("X-Bridge-Timestamp", timestamp)
	req.Header.Set("X-Bridge-Nonce", nonce)
	req.Header.Set("X-Bridge-Signature", base64.StdEncoding.EncodeToString(signature))
	return nil
}

func randomNonce() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
