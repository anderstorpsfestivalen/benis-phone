package sip

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/anderstorpsfestivalen/benis-phone/core/controller"
	"github.com/anderstorpsfestivalen/benis-phone/core/functions"
	"github.com/anderstorpsfestivalen/benis-phone/core/polly"
	"github.com/emiago/diago"
	"github.com/emiago/diago/media"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ClientConfig holds SIP client configuration for registering with a PBX.
type ClientConfig struct {
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
}

// Client handles SIP registration and incoming calls as a PBX extension.
type Client struct {
	config  ClientConfig
	ua      *sipgo.UserAgent
	diago   *diago.Diago
	manager *controller.SessionManager
	polly   polly.Polly
	def     functions.Definition

	regTx *diago.RegisterTransaction

	ctx    context.Context
	cancel context.CancelFunc

	activeCalls map[string]*callContext
	mu          sync.Mutex

	registered bool
	regMu      sync.RWMutex
}

// callContext holds per-call resources
type callContext struct {
	session    *controller.Session
	sipPhone   *SIPPhone
	audioSink  *RTPAudioSink
	audioSrc   *RTPAudioSource
	cancelFunc context.CancelFunc
}

// NewClient creates a new SIP client that will register with a PBX.
func NewClient(config ClientConfig, polly polly.Polly, def functions.Definition, maxCalls int) (*Client, error) {
	// Set defaults
	if config.Transport == "" {
		config.Transport = "udp"
	}
	if config.LocalPort == 0 {
		config.LocalPort = 5060
	}
	if config.ExpirySeconds == 0 {
		config.ExpirySeconds = 300
	}
	if config.Username == "" {
		config.Username = config.Extension
	}
	if config.Domain == "" {
		// Extract domain from server address
		host, _, err := net.SplitHostPort(config.Server)
		if err != nil {
			config.Domain = config.Server
		} else {
			config.Domain = host
		}
	}

	// Create user agent with the extension as the SIP user and domain as hostname
	ua, err := sipgo.NewUA(
		sipgo.WithUserAgent(config.Extension),
		sipgo.WithUserAgentHostname(config.Domain),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SIP user agent: %w", err)
	}

	// Detect local IP that can reach the SIP server
	localIP, err := getOutboundIP(config.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to detect local IP: %w", err)
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

	// Create session manager
	manager := controller.NewSessionManager(polly, def, maxCalls)

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		config:      config,
		ua:          ua,
		diago:       dg,
		manager:     manager,
		polly:       polly,
		def:         def,
		ctx:         ctx,
		cancel:      cancel,
		activeCalls: make(map[string]*callContext),
	}, nil
}

// Start registers with the PBX and begins listening for calls.
func (c *Client) Start() error {
	log.WithFields(log.Fields{
		"server":    c.config.Server,
		"extension": c.config.Extension,
		"domain":    c.config.Domain,
		"transport": c.config.Transport,
	}).Info("Starting SIP client")

	// Build the registrar URI
	registrarURI := sip.Uri{
		User: c.config.Extension,
		Host: c.config.Domain,
	}

	regOpts := diago.RegisterOptions{
		Username:  c.config.Username,
		Password:  c.config.Password,
		ProxyHost: c.config.Server,
		Expiry:    time.Duration(c.config.ExpirySeconds) * time.Second,
		OnRegistered: func() {
			c.regMu.Lock()
			c.registered = true
			c.regMu.Unlock()
			log.WithFields(log.Fields{
				"extension": c.config.Extension,
				"server":    c.config.Server,
			}).Info("Successfully registered with PBX")
		},
	}

	// Start serving incoming calls first (this sets up the transport)
	// ServeBackground waits for the listener to be ready before returning
	if err := c.diago.ServeBackground(c.ctx, c.handleIncomingCall); err != nil {
		return fmt.Errorf("failed to start SIP server: %w", err)
	}

	// Now start registration (transport is ready)
	go func() {
		err := c.diago.Register(c.ctx, registrarURI, regOpts)
		if err != nil && err != context.Canceled {
			log.WithError(err).Error("Registration failed")
			c.regMu.Lock()
			c.registered = false
			c.regMu.Unlock()
		}
	}()

	return nil
}

// Stop gracefully stops the SIP client and unregisters.
func (c *Client) Stop() {
	log.Info("Stopping SIP client")

	// Cancel context to stop registration loop and call handling
	c.cancel()

	// Stop all active calls
	c.mu.Lock()
	for callID, cc := range c.activeCalls {
		cc.cancelFunc()
		delete(c.activeCalls, callID)
	}
	c.mu.Unlock()

	// Stop all sessions via manager
	c.manager.StopAll()

	c.regMu.Lock()
	c.registered = false
	c.regMu.Unlock()

	log.Info("SIP client stopped")
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

	log.WithFields(log.Fields{
		"call_id":   callID,
		"from":      dialog.FromUser(),
		"to":        dialog.ToUser(),
		"transport": dialog.Transport(),
	}).Info("Incoming SIP call")

	// Send 100 Trying
	if err := dialog.Trying(); err != nil {
		log.WithError(err).Error("Failed to send Trying")
		return
	}

	// Send 180 Ringing
	if err := dialog.Ringing(); err != nil {
		log.WithError(err).Error("Failed to send Ringing")
		return
	}

	// Small delay to simulate ringing
	time.Sleep(500 * time.Millisecond)

	// Answer the call with NAT traversal enabled and explicit codec selection
	// RTPNATSymetric (1) learns the actual source address from incoming packets
	// which is essential for NAT traversal scenarios
	// Explicitly prefer PCMU/PCMA (8kHz) codecs to match our transcoded audio
	answerOpts := diago.AnswerOptions{
		RTPNAT: media.RTPNATSymetric,
		Codecs: []media.Codec{
			media.CodecAudioUlaw,          // PCMU - 8kHz
			media.CodecAudioAlaw,          // PCMA - 8kHz
			media.CodecTelephoneEvent8000, // DTMF
		},
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

	audioSrc := NewRTPAudioSource(dialog, c.config.RecordPath)

	// Create session via manager
	session, err := c.manager.CreateSession(callID, sipPhone, audioSink, audioSrc)
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
		session:    session,
		sipPhone:   sipPhone,
		audioSink:  audioSink,
		audioSrc:   audioSrc,
		cancelFunc: cancelFunc,
	}

	c.mu.Lock()
	c.activeCalls[callID] = cc
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
	return c.manager.ActiveSessionCount()
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
