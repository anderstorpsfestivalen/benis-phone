package bridge

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type EnrollmentClient struct {
	BaseURL string
	HTTP    *http.Client
	Version string
}

type EnrollmentStatus struct {
	RequestID string `json:"request_id"`
	Status    string `json:"status"`
	ExpiresAt int64  `json:"expires_at"`
	BridgeID  string `json:"bridge_id,omitempty"`
}

type EnrollmentHTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (err *EnrollmentHTTPError) Error() string {
	return fmt.Sprintf("enrollment endpoint returned %s: %s", err.Status, err.Body)
}

func EnrollmentCanonical(requestID, registrationID, publicKey, hostname, platform, version, timestamp string) string {
	return strings.Join([]string{
		"benis-phone-enrollment-v1",
		requestID,
		registrationID,
		publicKey,
		hostname,
		platform,
		version,
		timestamp,
	}, "\n")
}

func (c *EnrollmentClient) Submit(ctx context.Context, identity Identity, hostname, platform string) (EnrollmentStatus, error) {
	publicKey := base64.StdEncoding.EncodeToString(identity.PublicKey)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	version := c.Version
	if version == "" {
		version = "dev"
	}
	canonical := EnrollmentCanonical(
		identity.RequestID,
		identity.RegistrationID,
		publicKey,
		hostname,
		platform,
		version,
		timestamp,
	)
	signature := ed25519.Sign(ed25519.PrivateKey(identity.PrivateKey), []byte(canonical))
	payload, err := json.Marshal(map[string]string{
		"registration_id": identity.RegistrationID,
		"request_id":      identity.RequestID,
		"public_key":      publicKey,
		"hostname":        hostname,
		"platform":        platform,
		"version":         version,
		"timestamp":       timestamp,
		"signature":       base64.StdEncoding.EncodeToString(signature),
	})
	if err != nil {
		return EnrollmentStatus{}, err
	}
	return c.request(ctx, http.MethodPost, "/bridge/enroll", payload)
}

func (c *EnrollmentClient) Poll(ctx context.Context, requestID string) (EnrollmentStatus, error) {
	return c.request(ctx, http.MethodGet, "/bridge/enroll/"+url.PathEscape(requestID), nil)
}

func (c *EnrollmentClient) WaitForApproval(
	ctx context.Context,
	identity Identity,
	hostname string,
	platform string,
	pollInterval time.Duration,
) (string, error) {
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	var status EnrollmentStatus
	for {
		var err error
		status, err = c.Submit(ctx, identity, hostname, platform)
		if err == nil {
			break
		}
		var httpErr *EnrollmentHTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
			return "", err
		}
		if err := wait(ctx, pollInterval); err != nil {
			return "", err
		}
	}
	for {
		switch status.Status {
		case "approved":
			if status.BridgeID == "" {
				return "", fmt.Errorf("approved enrollment is missing bridge id")
			}
			return status.BridgeID, nil
		case "denied":
			return "", fmt.Errorf("bridge enrollment was denied")
		case "expired":
			return "", fmt.Errorf("bridge enrollment expired")
		case "pending":
		default:
			return "", fmt.Errorf("unknown enrollment status %q", status.Status)
		}
		if err := wait(ctx, pollInterval); err != nil {
			return "", err
		}
		var err error
		status, err = c.Poll(ctx, status.RequestID)
		if err != nil {
			continue
		}
	}
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *EnrollmentClient) request(ctx context.Context, method, path string, body []byte) (EnrollmentStatus, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return EnrollmentStatus{}, fmt.Errorf("invalid remote URL: %w", err)
	}
	base.Path = path
	base.RawQuery = ""
	req, err := http.NewRequestWithContext(ctx, method, base.String(), bytes.NewReader(body))
	if err != nil {
		return EnrollmentStatus{}, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return EnrollmentStatus{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return EnrollmentStatus{}, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return EnrollmentStatus{}, &EnrollmentHTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       truncate(data, 300),
		}
	}
	var status EnrollmentStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return EnrollmentStatus{}, fmt.Errorf("decode enrollment response: %w", err)
	}
	return status, nil
}

func truncate(value []byte, length int) string {
	value = bytes.TrimSpace(value)
	if len(value) <= length {
		return string(value)
	}
	return string(value[:length]) + "..."
}
