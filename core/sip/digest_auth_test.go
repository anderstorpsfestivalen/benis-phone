package sip

import (
	"testing"
	"time"

	"github.com/emiago/sipgo/sip"
	"github.com/icholy/digest"
)

func TestSIPDigestAuthenticatorAcceptsValidCredentialsOnce(t *testing.T) {
	auth := newSIPDigestAuthenticator()
	recipient := sip.Uri{Scheme: "sip", User: "46370123456", Host: "192.0.2.20", Port: 5062}

	challengeRequest := sip.NewRequest(sip.INVITE, recipient)
	authorized, response, err := auth.authorize(challengeRequest, "asterisk", "secret", "ivr.local")
	if err != nil || authorized || response == nil || response.StatusCode != sip.StatusUnauthorized {
		t.Fatalf("initial challenge = authorized %v, response %#v, err %v", authorized, response, err)
	}
	headers := response.GetHeaders("WWW-Authenticate")
	if len(headers) != 2 {
		t.Fatalf("401 response has %d challenges, want SHA-256 and MD5", len(headers))
	}
	challenge, err := digest.ParseChallenge(headers[0].Value())
	if err != nil {
		t.Fatalf("parse challenge: %v", err)
	}
	fallback, err := digest.ParseChallenge(headers[1].Value())
	if err != nil {
		t.Fatalf("parse fallback challenge: %v", err)
	}
	if challenge.Algorithm != "SHA-256" || fallback.Algorithm != "MD5" || !challenge.SupportsQOP("auth") {
		t.Fatalf("challenges = %#v / %#v, want SHA-256 then MD5 with qop=auth", challenge, fallback)
	}

	credentials, err := digest.Digest(challenge, digest.Options{
		Method:   sip.INVITE.String(),
		URI:      recipient.String(),
		Count:    1,
		Username: "asterisk",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("create credentials: %v", err)
	}
	authorizedRequest := sip.NewRequest(sip.INVITE, recipient)
	authorizedRequest.AppendHeader(sip.NewHeader("Authorization", credentials.String()))
	authorized, response, err = auth.authorize(authorizedRequest, "asterisk", "secret", "ivr.local")
	if err != nil || !authorized || response != nil {
		t.Fatalf("valid response = authorized %v, response %#v, err %v", authorized, response, err)
	}

	// The same Authorization header cannot start another transaction.
	replay := sip.NewRequest(sip.INVITE, recipient)
	replay.AppendHeader(sip.NewHeader("Authorization", credentials.String()))
	authorized, response, err = auth.authorize(replay, "asterisk", "secret", "ivr.local")
	if authorized || response == nil || response.StatusCode != sip.StatusUnauthorized || err == nil {
		t.Fatalf("replay = authorized %v, response %#v, err %v", authorized, response, err)
	}
}

func TestSIPDigestAuthenticatorRejectsWrongPasswordAndExpiredNonce(t *testing.T) {
	auth := newSIPDigestAuthenticator()
	now := time.Unix(1_700_000_000, 0)
	auth.now = func() time.Time { return now }
	recipient := sip.Uri{Scheme: "sip", User: "100", Host: "ivr.local"}

	challengeRequest := sip.NewRequest(sip.INVITE, recipient)
	_, response, err := auth.authorize(challengeRequest, "asterisk", "correct", "ivr.local")
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	challenge, err := digest.ParseChallenge(response.GetHeader("WWW-Authenticate").Value())
	if err != nil {
		t.Fatalf("parse challenge: %v", err)
	}
	wrong, err := digest.Digest(challenge, digest.Options{
		Method: sip.INVITE.String(), URI: recipient.String(), Count: 1,
		Username: "asterisk", Password: "wrong",
	})
	if err != nil {
		t.Fatalf("create wrong credentials: %v", err)
	}
	wrongRequest := sip.NewRequest(sip.INVITE, recipient)
	wrongRequest.AppendHeader(sip.NewHeader("Authorization", wrong.String()))
	authorized, response, err := auth.authorize(wrongRequest, "asterisk", "correct", "ivr.local")
	if authorized || response == nil || response.StatusCode != sip.StatusUnauthorized || err == nil {
		t.Fatalf("wrong password = authorized %v, response %#v, err %v", authorized, response, err)
	}

	// A separately issued challenge is rejected after the configured lifetime.
	_, response, err = auth.authorize(sip.NewRequest(sip.INVITE, recipient), "asterisk", "correct", "ivr.local")
	if err != nil {
		t.Fatalf("second challenge: %v", err)
	}
	expiring, err := digest.ParseChallenge(response.GetHeader("WWW-Authenticate").Value())
	if err != nil {
		t.Fatalf("parse expiring challenge: %v", err)
	}
	credentials, err := digest.Digest(expiring, digest.Options{
		Method: sip.INVITE.String(), URI: recipient.String(), Count: 1,
		Username: "asterisk", Password: "correct",
	})
	if err != nil {
		t.Fatalf("create expiring credentials: %v", err)
	}
	now = now.Add(sipDigestNonceTTL + time.Second)
	expiredRequest := sip.NewRequest(sip.INVITE, recipient)
	expiredRequest.AppendHeader(sip.NewHeader("Authorization", credentials.String()))
	authorized, response, err = auth.authorize(expiredRequest, "asterisk", "correct", "ivr.local")
	if authorized || response == nil || response.StatusCode != sip.StatusUnauthorized || err == nil {
		t.Fatalf("expired nonce = authorized %v, response %#v, err %v", authorized, response, err)
	}
	stale, err := digest.ParseChallenge(response.GetHeader("WWW-Authenticate").Value())
	if err != nil || !stale.Stale {
		t.Fatalf("expired nonce challenge = %#v, err %v, want stale=true", stale, err)
	}
}
