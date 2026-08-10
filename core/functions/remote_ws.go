package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/sirupsen/logrus"
)

// WSWatcher maintains a long-lived WebSocket subscription to the worker's
// ConfigBroker Durable Object. When the broker pushes a `config-updated`
// event for our config name, it invokes OnUpdate so the caller can pull
// the new TOML (the existing reloadOnce flow). On disconnect it reconnects
// with exponential backoff up to a 60-second cap.
type WSWatcher struct {
	BaseURL    string // e.g. "https://ivr.anderstorpsfestivalen.se"
	Name       string // config name we care about
	Token      string // bearer (PBXConfigToken)
	InstanceID string
	// Outgoing carries already-encoded control-plane messages such as
	// per-connection SIP status events.
	Outgoing <-chan []byte
	// Snapshot returns the latest encoded status for every connection. It is
	// replayed after runtime-hello on each connection so queue drops or a
	// disconnected runtime cannot leave the control plane permanently stale.
	Snapshot func() [][]byte
	// OnUpdate is invoked once per broker event matching Name. The hash
	// is supplied for logging / dedupe but the caller is expected to
	// re-fetch via the existing RemoteClient.
	OnUpdate func(hash string)
	Logger   *logrus.Logger
}

// Run blocks until ctx is cancelled, reconnecting between attempts.
func (w *WSWatcher) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := w.runOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			w.Logger.WithError(err).Warn("config-ws disconnected; reconnecting")
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
		// A successful connection resets backoff. We do that inside
		// runOnce by setting backoff = time.Second once the read loop
		// starts; not strictly needed for correctness so we keep the
		// caller-side simple geometric growth.
	}
}

func (w *WSWatcher) runOnce(ctx context.Context) error {
	u, err := buildWSURL(w.BaseURL, w.Name, w.InstanceID)
	if err != nil {
		return err
	}

	dialCtx, cancelDial := context.WithTimeout(ctx, 15*time.Second)
	defer cancelDial()
	c, _, err := websocket.Dial(dialCtx, u, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + w.Token}},
	})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.CloseNow()

	w.Logger.WithFields(logrus.Fields{
		"url":  u,
		"name": w.Name,
	}).Info("config-ws connected")

	// One writer owns all outgoing frames: status events and a heartbeat that
	// lets the UI distinguish an idle healthy runtime from an offline one.
	pingCtx, cancelPing := context.WithCancel(ctx)
	defer cancelPing()
	writeErr := make(chan error, 1)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		write := func(data []byte) bool {
			writeCtx, cancel := context.WithTimeout(pingCtx, 10*time.Second)
			err := c.Write(writeCtx, websocket.MessageText, data)
			cancel()
			if err != nil {
				select {
				case writeErr <- err:
				default:
				}
				_ = c.Close(websocket.StatusInternalError, "write failed")
				return false
			}
			return true
		}
		for _, data := range w.initialMessages() {
			if !write(data) {
				return
			}
		}
		for {
			select {
			case <-pingCtx.Done():
				return
			case <-t.C:
				heartbeat, _ := json.Marshal(map[string]any{
					"type": "heartbeat", "instance_id": w.InstanceID,
					"at": time.Now().UTC().Format(time.RFC3339Nano),
				})
				if !write(heartbeat) {
					return
				}
			case data, ok := <-w.Outgoing:
				if !ok {
					w.Outgoing = nil
					continue
				}
				if !write(data) {
					return
				}
			}
		}
	}()

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			select {
			case writeErrValue := <-writeErr:
				return fmt.Errorf("write: %w", writeErrValue)
			default:
			}
			return fmt.Errorf("read: %w", err)
		}
		var msg struct {
			Type string `json:"type"`
			Name string `json:"name"`
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			w.Logger.WithError(err).WithField("raw", string(data)).Warn("config-ws: bad payload")
			continue
		}
		if msg.Type != "config-updated" || msg.Name != w.Name {
			continue
		}
		w.Logger.WithField("hash", shortHashStr(msg.Hash)).Info("config-ws update")
		if w.OnUpdate != nil {
			w.OnUpdate(msg.Hash)
		}
	}
}

func (w *WSWatcher) initialMessages() [][]byte {
	hello, _ := json.Marshal(map[string]any{
		"type": "runtime-hello", "instance_id": w.InstanceID,
		"at": time.Now().UTC().Format(time.RFC3339Nano),
	})
	messages := [][]byte{hello}
	if w.Snapshot != nil {
		messages = append(messages, w.Snapshot()...)
	}
	return messages
}

// buildWSURL turns "https://host" + name into "wss://host/config/ws?name=...".
func buildWSURL(baseURL, name, instanceID string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base url %q: %w", baseURL, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("base url scheme must be http or https, got %q", u.Scheme)
	}
	u.Path = "/config/ws"
	q := u.Query()
	q.Set("name", name)
	q.Set("instance_id", instanceID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func shortHashStr(h string) string {
	if len(h) < 8 {
		return h
	}
	return h[:8]
}
