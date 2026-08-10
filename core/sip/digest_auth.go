package sip

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
)

const sipDigestNonceTTL = 60 * time.Second

var errSIPDigestUnauthorized = errors.New("invalid SIP digest credentials")

var sipDigestAlgorithms = []string{"SHA-256", "MD5"}

type sipDigestNonce struct {
	challenge digest.Challenge
	expiresAt time.Time
}

// sipDigestAuthenticator challenges inbound INVITEs using standard SIP Digest
// authentication. Nonces are random, short-lived, and single-use so a captured
// Authorization header cannot be replayed as a new transaction.
type sipDigestAuthenticator struct {
	mu     sync.Mutex
	nonces map[string]sipDigestNonce
	now    func() time.Time
}

func newSIPDigestAuthenticator() *sipDigestAuthenticator {
	return &sipDigestAuthenticator{
		nonces: make(map[string]sipDigestNonce),
		now:    time.Now,
	}
}

// authorize returns authorized=true only after a valid Digest response. Every
// other normal authentication outcome includes a response that must be written
// to the INVITE transaction (normally a fresh 401 challenge).
func (a *sipDigestAuthenticator) authorize(req *sip.Request, username, password, realm string) (authorized bool, response *sip.Response, err error) {
	header := req.GetHeader("Authorization")
	if header == nil {
		response, err = a.challenge(req, realm, false)
		return false, response, err
	}

	credentials, parseErr := digest.ParseCredentials(header.Value())
	if parseErr != nil {
		response, err = a.challenge(req, realm, false)
		if err != nil {
			return false, response, err
		}
		return false, response, errors.Join(errSIPDigestUnauthorized, parseErr)
	}

	nonce, ok := a.consumeNonce(credentials.Nonce)
	if !ok {
		response, err = a.challenge(req, realm, true)
		if err != nil {
			return false, response, err
		}
		return false, response, errSIPDigestUnauthorized
	}

	want, digestErr := digest.Digest(&nonce.challenge, digest.Options{
		Method:   req.Method.String(),
		URI:      credentials.URI,
		Count:    credentials.Nc,
		Username: username,
		Password: password,
		Cnonce:   credentials.Cnonce,
	})
	valid := digestErr == nil &&
		constantTimeStringEqual(credentials.Username, username) &&
		constantTimeStringEqual(credentials.Realm, nonce.challenge.Realm) &&
		credentials.URI == req.Recipient.String() &&
		digestAlgorithmMatches(credentials.Algorithm, nonce.challenge.Algorithm) &&
		credentials.QOP == "auth" && credentials.Nc > 0 && credentials.Cnonce != "" &&
		constantTimeStringEqual(credentials.Response, wantResponse(want))
	if valid {
		return true, nil, nil
	}

	response, err = a.challenge(req, realm, false)
	if err != nil {
		return false, response, err
	}
	return false, response, errors.Join(errSIPDigestUnauthorized, digestErr)
}

func (a *sipDigestAuthenticator) challenge(req *sip.Request, realm string, stale bool) (*sip.Response, error) {
	challenges := make([]digest.Challenge, 0, len(sipDigestAlgorithms))
	for _, algorithm := range sipDigestAlgorithms {
		nonceBytes := make([]byte, 32)
		if _, err := rand.Read(nonceBytes); err != nil {
			return sip.NewResponseFromRequest(req, sip.StatusInternalServerError, "Internal Server Error", nil), err
		}
		challenges = append(challenges, digest.Challenge{
			Realm:     realm,
			Nonce:     base64.RawURLEncoding.EncodeToString(nonceBytes),
			Stale:     stale,
			Algorithm: algorithm,
			QOP:       []string{"auth"},
		})
	}
	now := a.now()
	a.mu.Lock()
	for key, entry := range a.nonces {
		if !entry.expiresAt.After(now) {
			delete(a.nonces, key)
		}
	}
	for _, challenge := range challenges {
		a.nonces[challenge.Nonce] = sipDigestNonce{challenge: challenge, expiresAt: now.Add(sipDigestNonceTTL)}
	}
	a.mu.Unlock()

	response := sip.NewResponseFromRequest(req, sip.StatusUnauthorized, "Unauthorized", nil)
	for _, challenge := range challenges {
		response.AppendHeader(sip.NewHeader("WWW-Authenticate", challenge.String()))
	}
	return response, nil
}

func (a *sipDigestAuthenticator) consumeNonce(nonce string) (sipDigestNonce, bool) {
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.nonces[nonce]
	delete(a.nonces, nonce)
	if !ok || !entry.expiresAt.After(now) {
		return sipDigestNonce{}, false
	}
	return entry, true
}

func constantTimeStringEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func digestAlgorithmMatches(got, want string) bool {
	if got == "" {
		got = "MD5"
	}
	return strings.EqualFold(got, want)
}

func wantResponse(credentials *digest.Credentials) string {
	if credentials == nil {
		return ""
	}
	return credentials.Response
}
