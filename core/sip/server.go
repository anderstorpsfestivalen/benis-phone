package sip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/controller"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/emiago/diago"
	"github.com/emiago/diago/media"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// telephoneEventPT100 is Linphone's payload-type assignment for 8 kHz
// telephone-event DTMF. diago only ships a PT-101 variant; we add this so
// negotiation accepts Linphone (and similar clients) and our SDP answer
// echoes the PT they offered.
var telephoneEventPT100 = media.Codec{
	PayloadType: 100,
	SampleRate:  8000,
	SampleDur:   20 * time.Millisecond,
	NumChannels: 1,
	Name:        "telephone-event",
}

// ClientConfig holds SIP client configuration for registering with a PBX.
type ClientConfig struct {
	ConnectionID string
	Name         string
	Kind         string
	Registration string
	Entrypoint   string
	Routes       []functions.SIPRoute
	AllowedCIDRs []string

	// Server is the SIP server/PBX address (e.g., "pbx.example.com:5060")
	Server string

	// Extension is the extension number to register as
	Extension string

	// Username for SIP authentication
	Username string

	// Password for SIP authentication
	Password string

	// Domain is the SIP domain
	Domain string

	// Transport is the SIP transport (udp, tcp)
	Transport string

	// LocalPort is the local port to bind to
	LocalPort int

	// ExpirySeconds is the registration expiry time
	ExpirySeconds int

	// RecordPath is the base path for call recordings
	RecordPath string

	// ExternalIP is the public IP for NAT traversal (used in SDP for RTP)
	ExternalIP string

	// InboundOnly skips REGISTER. Calls must pass Digest auth and AllowedCIDRs.
	InboundOnly bool
}

// Client handles SIP registration and incoming calls as a PBX extension.
type Client struct {
	configMu   sync.RWMutex
	config     ClientConfig
	definition functions.Definition
	ua         *sipgo.UserAgent
	diago      *diago.Diago
	manager    *controller.SessionManager
	regCancel  context.CancelFunc
	regLifeMu  sync.Mutex
	regWG      sync.WaitGroup
	stopOnce   sync.Once

	ctx    context.Context
	cancel context.CancelFunc

	activeCalls  map[string]*callContext
	pendingCalls int
	mu           sync.Mutex

	registered bool
	regMu      sync.RWMutex

	accepting  bool
	statusMu   sync.RWMutex
	status     func(StatusEvent)
	digestAuth *sipDigestAuthenticator
}

// callContext holds per-call resources
type callContext struct {
	session     *controller.Session
	dialog      *diago.DialogServerSession
	sipPhone    *SIPPhone
	audioSink   *RTPAudioSink
	audioSrc    *RTPAudioSource
	callCtl     *sipController
	cancelFunc  context.CancelFunc
	cleanupDone chan struct{}
}

// NewClient creates a new SIP client that will register with a PBX.
func NewClient(config ClientConfig, def functions.Definition, manager *controller.SessionManager, status func(StatusEvent)) (*Client, error) {
	config = normalizeClientConfig(config)
	if config.LocalPort == 0 {
		return nil, fmt.Errorf("local port must be allocated before creating SIP client")
	}

	// Create user agent with the extension as the SIP user and domain as hostname
	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent(config.Extension),
		sipgo.WithUserAgentHostname(config.Domain),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP user agent: %w", err)
	}

	// Detect local IP for binding/SDP. In inbound-only mode there's no server to
	// route towards, so we just discover a reasonable LAN IP.
	var localIP string
	if config.InboundOnly {
		localIP, err = getLANIP()
		if err != nil {
			return nil, fmt.Errorf("failed to detect local IP: %w", err)
		}
	} else {
		localIP, err = getOutboundIP(config.Server)
		if err != nil {
			return nil, fmt.Errorf("failed to detect local IP: %w", err)
		}
	}

	log.WithField("local_ip", localIP).Info("Detected local IP for SIP")

	// Configure transport - bind to the detected local IP directly
	transport := diago.Transport{
		Transport: config.Transport,
		BindHost:  localIP,
		BindPort:  config.LocalPort,
	}

	// If external IP is configured, use it for NAT traversal
	if config.ExternalIP != "" {
		extIP := net.ParseIP(config.ExternalIP)
		if extIP == nil {
			return nil, fmt.Errorf("invalid external IP: %s", config.ExternalIP)
		}
		transport.ExternalHost = config.ExternalIP
		transport.MediaExternalIP = extIP
		log.WithField("external_ip", config.ExternalIP).Info("Using external IP for NAT traversal")
	}

	// Pre-create the sipgo client bound to the same local addr the listener will use,
	// so REGISTER and other outbound requests share the listening socket. Without this,
	// diago creates its per-transport client before the listener starts, falls back to
	// an ephemeral source port, and PBX responses (sent to Via host:port) can't be
	// associated with the outbound transaction.
	clientOpts := []sipgo.ClientOption{
		sipgo.WithClientNAT(),
		sipgo.WithClientConnectionAddr(net.JoinHostPort(localIP, strconv.Itoa(config.LocalPort))),
	}
	if config.ExternalIP != "" {
		// Make outgoing Via headers advertise the external IP:port pair.
		clientOpts = append(clientOpts,
			sipgo.WithClientHostname(config.ExternalIP),
			sipgo.WithClientPort(config.LocalPort),
		)
	}

	sipClient, err := sipgo.NewClient(ua, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP client: %w", err)
	}

	dg := diago.NewDiago(ua,
		diago.WithTransport(transport),
		diago.WithClient(sipClient),
	)

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:      config,
		definition:  def,
		ua:          ua,
		diago:       dg,
		manager:     manager,
		ctx:         ctx,
		cancel:      cancel,
		activeCalls: make(map[string]*callContext),
		accepting:   true,
		status:      status,
		digestAuth:  newSIPDigestAuthenticator(),
	}, nil
}

func normalizeClientConfig(config ClientConfig) ClientConfig {
	config.Transport = strings.ToLower(strings.TrimSpace(config.Transport))
	if config.Transport == "" {
		config.Transport = "udp"
	}
	if config.ExpirySeconds == 0 {
		config.ExpirySeconds = 300
	}
	if config.Extension == "" {
		config.Extension = config.ConnectionID
	}
	if config.Username == "" {
		config.Username = config.Extension
	}
	if config.Domain == "" {
		if config.InboundOnly {
			config.Domain = "local"
		} else {
			host, _, err := net.SplitHostPort(config.Server)
			if err != nil {
				config.Domain = config.Server
			} else {
				config.Domain = host
			}
		}
	}
	return config
}

// SessionManager returns the controller managing per-call sessions. Used
// by the hot-reload loop in main to swap the active Definition.
func (c *Client) SessionManager() *controller.SessionManager {
	return c.manager
}

// Start registers with the PBX (or only listens for an inbound connection) and begins
// accepting calls.
func (c *Client) Start() error {
	cfg := c.Config()
	c.emit("starting", "", "starting SIP listener")
	log.WithFields(log.Fields{
		"connection": cfg.ConnectionID,
		"server":     cfg.Server,
		"extension":  cfg.Extension,
		"domain":     cfg.Domain,
		"transport":  cfg.Transport,
		"inbound":    cfg.InboundOnly,
	}).Info("Starting SIP client")

	// Start serving incoming calls first (this sets up the transport).
	// ServeBackground waits for the listener to be ready before returning.
	if err := c.diago.ServeBackground(c.ctx, c.handleIncomingCall); err != nil {
		c.emit("error", "bind_failed", err.Error())
		return fmt.Errorf("failed to start SIP server: %w", err)
	}

	if cfg.InboundOnly {
		c.regMu.Lock()
		c.registered = true
		c.regMu.Unlock()
		log.WithFields(log.Fields{
			"connection": cfg.ConnectionID,
			"bind_port":  cfg.LocalPort,
			"transport":  cfg.Transport,
		}).Info("Inbound SIP connection listening (no REGISTER)")
		c.emit("listening", "", "inbound listener ready")
		return nil
	}
	c.restartRegistration()
	return nil
}

func (c *Client) restartRegistration() {
	c.regLifeMu.Lock()
	defer c.regLifeMu.Unlock()
	c.stopRegistrationLocked()
	if c.ctx.Err() != nil {
		return
	}
	cfg := c.Config()
	c.regMu.Lock()
	regCtx, cancel := context.WithCancel(c.ctx)
	c.regCancel = cancel
	c.registered = false
	c.regMu.Unlock()

	if cfg.InboundOnly {
		c.regMu.Lock()
		c.registered = true
		c.regMu.Unlock()
		c.emit("listening", "", "inbound listener ready")
		return
	}
	c.emit("registering", "", "registering with upstream PBX")
	registrarURI := sip.Uri{
		User:      cfg.Extension,
		Host:      cfg.Domain,
		UriParams: sip.HeaderParams{"transport": cfg.Transport},
	}

	regOpts := diago.RegisterOptions{
		Username:  cfg.Username,
		Password:  cfg.Password,
		ProxyHost: cfg.Server,
		Expiry:    time.Duration(cfg.ExpirySeconds) * time.Second,
		OnRegistered: func() {
			c.regMu.Lock()
			c.registered = true
			c.regMu.Unlock()
			log.WithFields(log.Fields{
				"connection": cfg.ConnectionID,
				"extension":  cfg.Extension,
				"server":     cfg.Server,
			}).Info("Successfully registered with PBX")
			c.emit("ready", "", "registered with upstream PBX")
		},
	}

	c.regWG.Add(1)
	go func() {
		defer c.regWG.Done()
		for regCtx.Err() == nil {
			err := c.diago.Register(regCtx, registrarURI, regOpts)
			if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			log.WithError(err).Error("Registration failed")
			c.regMu.Lock()
			c.registered = false
			c.regMu.Unlock()
			code := "registration_failed"
			var responseErr *diago.RegisterResponseError
			if errors.As(err, &responseErr) && (responseErr.StatusCode() == 401 || responseErr.StatusCode() == 403 || responseErr.StatusCode() == 407) {
				code = "auth_failed"
			}
			c.emit("error", code, err.Error())
			select {
			case <-regCtx.Done():
				return
			case <-time.After(30 * time.Second):
				c.emit("registering", "retry", "retrying registration")
			}
		}
	}()
}

func (c *Client) stopRegistration() {
	c.regLifeMu.Lock()
	defer c.regLifeMu.Unlock()
	c.stopRegistrationLocked()
}

func (c *Client) stopRegistrationLocked() {
	c.regMu.Lock()
	if c.regCancel != nil {
		c.regCancel()
		c.regCancel = nil
	}
	c.regMu.Unlock()
	c.regWG.Wait()
}

// Config returns a copy of the current connection configuration.
func (c *Client) Config() ClientConfig {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.config
}

// beginIncomingCall captures one coherent listener snapshot and reserves a
// call slot before Drain can observe the listener as idle. Without the pending
// reservation, a call already inside diago but not yet in activeCalls could be
// torn down underneath its answer/session setup.
func (c *Client) beginIncomingCall() (ClientConfig, functions.Definition, bool) {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	if !c.accepting {
		return ClientConfig{}, functions.Definition{}, false
	}
	c.mu.Lock()
	c.pendingCalls++
	c.mu.Unlock()
	return c.config, c.definition, true
}

func (c *Client) releasePendingCall() {
	c.mu.Lock()
	c.pendingCalls--
	c.mu.Unlock()
}

// Update changes routing and registration settings without restarting the
// listener. The supervisor only calls this when bind/NAT settings are stable.
func (c *Client) Update(cfg ClientConfig, def functions.Definition, restartRegistration bool) {
	c.configMu.Lock()
	c.config = cfg
	c.definition = def
	c.configMu.Unlock()
	if restartRegistration {
		c.restartRegistration()
	}
}

func (c *Client) emit(state, code, message string) {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	if c.status == nil {
		return
	}
	cfg := c.Config()
	c.status(newStatus(cfg.ConnectionID, state, code, message, cfg.LocalPort))
}

// silenceStatus prevents a superseded listener from overwriting the status
// emitted by its live replacement while the old listener drains calls.
func (c *Client) silenceStatus() {
	c.statusMu.Lock()
	c.status = nil
	c.statusMu.Unlock()
}

// Stop gracefully stops the SIP client and unregisters.
func (c *Client) Stop() {
	c.stopOnce.Do(func() {
		cfg := c.Config()
		log.WithField("connection", cfg.ConnectionID).Info("Stopping SIP client")

		// Cancel registration before closing the UA so diago can send its
		// best-effort unregister using the still-live transport.
		c.stopRegistration()
		c.cancel()

		// cleanupCall is idempotent through its activeCalls delete, so it is
		// safe if a dialog monitor is finishing at the same time.
		c.mu.Lock()
		calls := make(map[string]*callContext, len(c.activeCalls))
		for callID, cc := range c.activeCalls {
			calls[callID] = cc
		}
		c.mu.Unlock()
		for callID, cc := range calls {
			c.cleanupCall(callID, cc.dialog)
			<-cc.cleanupDone
		}

		// A supervisor may immediately bind a replacement to this port.
		// Closing the UA synchronously releases all SIP transports first.
		if err := c.ua.Close(); err != nil {
			log.WithError(err).WithField("connection", cfg.ConnectionID).Warn("Closing SIP user agent")
		}

		c.regMu.Lock()
		c.registered = false
		c.regMu.Unlock()

		c.emit("stopped", "", "SIP connection stopped")
		log.WithField("connection", cfg.ConnectionID).Info("SIP client stopped")
	})
}

// Drain prevents new calls and unregisters while keeping the listener and
// active dialogs alive. The supervisor calls Stop after ActiveCalls reaches 0.
func (c *Client) Drain() {
	c.configMu.Lock()
	c.accepting = false
	c.configMu.Unlock()
	c.stopRegistration()
	c.emit("draining", "", "waiting for active calls to finish")
}

// IsRegistered returns true if currently registered with the PBX.
func (c *Client) IsRegistered() bool {
	c.regMu.RLock()
	defer c.regMu.RUnlock()
	return c.registered
}

// handleIncomingCall is called for each incoming SIP INVITE.
func (c *Client) handleIncomingCall(dialog *diago.DialogServerSession) {
	callID := uuid.New().String()
	cfg, def, accepting := c.beginIncomingCall()
	if !accepting {
		_ = dialog.Respond(503, "Service Unavailable", nil)
		return
	}
	pending := true
	defer func() {
		if pending {
			c.releasePendingCall()
		}
	}()
	if !sourceAllowed(dialog.InviteRequest.Source(), cfg.AllowedCIDRs) {
		log.WithFields(log.Fields{"connection": cfg.ConnectionID, "source": dialog.InviteRequest.Source()}).Warn("Rejected SIP INVITE outside allowed CIDRs")
		_ = dialog.Respond(403, "Forbidden", nil)
		return
	}
	if cfg.InboundOnly {
		authorized, response, err := c.digestAuth.authorize(
			dialog.InviteRequest,
			cfg.Username,
			cfg.Password,
			cfg.Domain,
		)
		if !authorized {
			fields := log.Fields{"connection": cfg.ConnectionID, "source": dialog.InviteRequest.Source()}
			if err != nil {
				log.WithError(err).WithFields(fields).Warn("Rejected SIP INVITE with invalid digest credentials")
			}
			if response == nil {
				_ = dialog.Respond(sip.StatusInternalServerError, "Internal Server Error", nil)
			} else if writeErr := dialog.WriteResponse(response); writeErr != nil {
				log.WithError(writeErr).WithFields(fields).Warn("Writing SIP digest challenge")
			}
			return
		}
	}
	called := dialog.InviteRequest.Recipient.User
	if called == "" {
		called = dialog.ToUser()
	}
	entrypoint, ok := resolveEntrypoint(cfg, called)
	if !ok {
		log.WithFields(log.Fields{"connection": cfg.ConnectionID, "called": called}).Info("No SIP route matched")
		_ = dialog.Respond(404, "Not Found", nil)
		return
	}

	log.WithFields(log.Fields{
		"call_id":    callID,
		"connection": cfg.ConnectionID,
		"from":       dialog.FromUser(),
		"to":         dialog.ToUser(),
		"called":     called,
		"entrypoint": entrypoint,
		"transport":  dialog.Transport(),
	}).Info("Incoming SIP call")

	// Send 100 Trying
	if err := dialog.Trying(); err != nil {
		log.WithError(err).Error("Failed to send Trying")
		return
	}

	if !cfg.InboundOnly {
		// Send 180 Ringing only when behind a real PBX. Some softphones in
		// direct peers sometimes CANCEL during the provisional window, which races
		// the 200 OK and produces "transaction terminated" on Answer.
		if err := dialog.Ringing(); err != nil {
			log.WithError(err).Error("Failed to send Ringing")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Explicitly prefer PCMU/PCMA (8kHz) codecs to match our transcoded audio.
	// We offer telephone-event/8000 at both PT 101 (the IANA-recommended
	// default that most PBXes use) and PT 100 (what Linphone offers).
	// diago's negotiation does strict struct-equality on codecs, so without
	// the second entry Linphone's offer is silently dropped and DTMF is dead.
	answerOpts := diago.AnswerOptions{
		Codecs: []media.Codec{
			media.CodecAudioUlaw,          // PCMU - 8kHz
			media.CodecAudioAlaw,          // PCMA - 8kHz
			media.CodecTelephoneEvent8000, // DTMF — PT 101
			telephoneEventPT100,           // DTMF — PT 100 (Linphone)
		},
	}
	if !cfg.InboundOnly {
		// RTPNATSymetric learns the remote RTP source from incoming packets —
		// essential behind NAT, harmful on a flat LAN where the offered SDP
		// already points at a reachable address.
		answerOpts.RTPNAT = media.RTPNATSymetric
	}
	if err := dialog.AnswerOptions(answerOpts); err != nil {
		log.WithError(err).Error("Failed to answer call")
		return
	}

	log.WithField("call_id", callID).Info("Call answered with RTP NAT symmetric mode")

	// Log media session details for debugging
	msess := dialog.MediaSession()
	if msess != nil {
		log.WithFields(log.Fields{
			"call_id":     callID,
			"local_addr":  msess.Laddr.String(),
			"remote_addr": msess.Raddr.String(),
			"mode":        msess.Mode,
			"rtp_nat":     msess.RTPNAT,
		}).Info("Media session established")
	} else {
		log.WithField("call_id", callID).Error("Media session is nil after answer!")
	}

	// Create per-call components
	sipPhone := NewSIPPhone(dialog)

	audioSink, err := NewRTPAudioSink(dialog)
	if err != nil {
		log.WithError(err).Error("Failed to create RTP audio sink")
		dialog.Hangup(c.ctx)
		return
	}

	audioSrc := NewRTPAudioSource(dialog, cfg.RecordPath)

	// Wire the per-call recorder so SIPPhone taps inbound and OutputStream
	// (inside audioSink) taps outbound. The sipController also holds a
	// reference so StartRecording/StopRecording flips the shared state.
	rec := audioSink.Recorder()
	sipPhone.SetRecorder(rec)

	callCtl := newSIPController(callID, dialog, cfg.RecordPath, cfg.Domain, rec)

	// Create session via manager
	session, err := c.manager.CreateSession(callID, entrypoint, def, sipPhone, audioSink, audioSrc, callCtl)
	if err != nil {
		log.WithError(err).Error("Failed to create session")
		dialog.Hangup(c.ctx)
		return
	}

	// Initialize the SIP phone (starts DTMF listening)
	if err := sipPhone.Init(); err != nil {
		log.WithError(err).Error("Failed to initialize SIP phone")
		c.manager.RemoveSession(callID)
		dialog.Hangup(c.ctx)
		return
	}

	// Create call context
	callCtx, cancelFunc := context.WithCancel(c.ctx)
	cc := &callContext{
		session:     session,
		dialog:      dialog,
		sipPhone:    sipPhone,
		audioSink:   audioSink,
		audioSrc:    audioSrc,
		callCtl:     callCtl,
		cancelFunc:  cancelFunc,
		cleanupDone: make(chan struct{}),
	}

	c.mu.Lock()
	c.activeCalls[callID] = cc
	c.pendingCalls--
	pending = false
	c.mu.Unlock()

	// Start session in background
	go c.runSession(callID, session, dialog, callCtx)

	// Monitor dialog state (wait for BYE)
	c.monitorDialog(callID, dialog, callCtx)
}

// runSession runs the IVR session for a call.
func (c *Client) runSession(callID string, session *controller.Session, dialog *diago.DialogServerSession, ctx context.Context) {
	// Start the session (this blocks until session ends)
	session.Start()
}

// monitorDialog monitors the SIP dialog for termination.
// Instead of using dialog.ListenContext() which would compete with DTMF reader,
// we wait for the SIPPhone's done channel which signals when the DTMF read loop ends.
func (c *Client) monitorDialog(callID string, dialog *diago.DialogServerSession, ctx context.Context) {
	c.mu.Lock()
	cc, exists := c.activeCalls[callID]
	c.mu.Unlock()

	if !exists {
		return
	}
	defer close(cc.cleanupDone)

	// Wait for either context cancellation or DTMF loop to end
	select {
	case <-ctx.Done():
		log.WithField("call_id", callID).Debug("Dialog context canceled")
	case <-cc.sipPhone.Done():
		log.WithField("call_id", callID).Debug("DTMF read loop ended (call terminated)")
	}

	// Call ended - cleanup
	c.cleanupCall(callID, dialog)
}

// cleanupCall removes a call and releases resources.
func (c *Client) cleanupCall(callID string, dialog *diago.DialogServerSession) {
	c.mu.Lock()
	cc, exists := c.activeCalls[callID]
	if exists {
		delete(c.activeCalls, callID)
	}
	c.mu.Unlock()

	if !exists {
		return
	}

	log.WithField("call_id", callID).Info("Cleaning up call")

	// Cancel call context
	cc.cancelFunc()

	// Close SIP phone (signals hook down)
	cc.sipPhone.Close()

	// Flush any in-progress recording before the OutputStream goroutine exits
	// (it owns the FeedOutbound tap). StopRecording is idempotent and returns
	// ErrNotRecording if nothing is active — safe to ignore that.
	if cc.callCtl != nil {
		_ = cc.callCtl.StopRecording()
	}

	// Stop the output stream goroutine before tearing down RTP. Outstanding
	// playAndWait callers receive ErrInterrupted from the drained queue.
	cc.audioSink.Close()

	// Stop recording
	cc.audioSrc.Stop()

	// Remove session from manager
	c.manager.RemoveSession(callID)

	// Hangup dialog if still active
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialog.Hangup(ctx)

	log.WithField("call_id", callID).Info("Call cleaned up")
}

// ActiveCalls returns the number of active calls.
func (c *Client) ActiveCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.activeCalls) + c.pendingCalls
}

func (c *Client) IsAccepting() bool {
	c.configMu.RLock()
	defer c.configMu.RUnlock()
	return c.accepting
}

func resolveEntrypoint(cfg ClientConfig, called string) (string, bool) {
	conn := functions.SIPConnection{
		Kind:       cfg.Kind,
		Entrypoint: cfg.Entrypoint,
		Routes:     cfg.Routes,
	}
	return conn.ResolveEntrypoint(called)
}

func sourceAllowed(source string, cidrs []string) bool {
	if len(cidrs) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(source)
	if err != nil {
		host = source
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(raw))
		if err == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

// getOutboundIP finds the local IP address that would be used to reach the given destination.
func getOutboundIP(dest string) (string, error) {
	// Parse destination to get host
	host, port, err := net.SplitHostPort(dest)
	if err != nil {
		// Maybe no port specified
		host = dest
		port = "5060"
	}

	// Resolve hostname to IP
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %s: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("no IP addresses found for %s", host)
	}

	// Use UDP dial to find the outbound IP (doesn't actually send anything)
	conn, err := net.Dial("udp", net.JoinHostPort(ips[0].String(), port))
	if err != nil {
		return "", fmt.Errorf("failed to determine outbound IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// getLANIP returns a plausible LAN IP for SDP/binding when there's no SIP
// server destination to probe against. Falls back to 127.0.0.1 if nothing
// routable is found.
func getLANIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1", nil
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}
