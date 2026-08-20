package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// skyStreamDialConfig holds what's needed to open a Sky Stream session,
// split out from the box's configured host/port so tests can point it at a
// local fake server instead of a real box.
type skyStreamDialConfig struct {
	// URL is the full WebSocket URL to dial, e.g. "wss://192.168.1.50:8091/iptarget".
	URL string
	// ServerName is the TLS SNI value the real protocol expects
	// ("sky.xcal.tv"). It has no effect on the connection's trust decision
	// since server certificate verification is always disabled below.
	ServerName string
	// Origin is sent as the WebSocket handshake's Origin header, matching
	// what the official app sends: "https://<device-ip>:8091/".
	Origin string
	Cert   tls.Certificate
}

// dialSkyStream opens the TLS+mTLS+WebSocket connection a Sky Stream
// session runs over. Server certificate verification is intentionally
// disabled (InsecureSkipVerify) - this mirrors the documented behavior of
// the official app itself (Docs/SKY_REMOTE_PROTOCOL.md section 3), not a
// testing shortcut: this protocol only verifies the client's identity (via
// the required client certificate), not the box's.
func dialSkyStream(ctx context.Context, cfg skyStreamDialConfig) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cfg.Cert},
			ServerName:         cfg.ServerName,
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS13,
			// The box's TLS stack expects ALPN to include http/1.1 (see
			// Docs/SKY_REMOTE_PROTOCOL.md section 3); without it the
			// handshake was observed failing with an abrupt EOF rather
			// than a clean TLS alert.
			NextProtos: []string{"http/1.1"},
		},
	}
	headers := http.Header{}
	if cfg.Origin != "" {
		headers.Set("Origin", cfg.Origin)
	}
	// These match what the official app sends (Docs/SKY_REMOTE_PROTOCOL.md
	// section 4) - the box's embedded WebSocket server was observed
	// closing the connection with an abrupt EOF (no HTTP response at all)
	// when the upgrade request was missing them.
	headers.Set("User-Agent", "Dart/3.9 (dart:io)")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Accept-Encoding", "gzip")

	conn, _, err := dialer.DialContext(ctx, cfg.URL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sky Stream box: %w", err)
	}
	return conn, nil
}

// skyStreamSession holds the pair/bind state for one WebSocket connection.
// Callers must call pairAndBind once after dialing, then sendKey for each
// button press; the bind_id is tied to the connection it was issued on, so
// a fresh session (new dial + pairAndBind) is needed after any error.
type skyStreamSession struct {
	conn            *websocket.Conn
	certFingerprint string

	tid       string
	authToken string
	bindID    int
}

func newSkyStreamSession(conn *websocket.Conn, cert tls.Certificate) (*skyStreamSession, error) {
	fp, err := skyStreamCertFingerprint(cert)
	if err != nil {
		return nil, err
	}
	return &skyStreamSession{conn: conn, certFingerprint: fp}, nil
}

// skyStreamCertFingerprint returns the lowercase hex SHA-256 fingerprint of
// a certificate's leaf DER encoding, as required by the auth token
// derivation (Docs/SKY_REMOTE_PROTOCOL.md section 6).
func skyStreamCertFingerprint(cert tls.Certificate) (string, error) {
	if len(cert.Certificate) == 0 {
		return "", fmt.Errorf("certificate has no leaf DER bytes")
	}
	sum := sha256.Sum256(cert.Certificate[0])
	return hex.EncodeToString(sum[:]), nil
}

// pairAndBind runs the Pair Request -> Pair Response -> Bind Request ->
// Bind Response sequence (Docs/SKY_REMOTE_PROTOCOL.md sections 5 and 7),
// leaving the session ready for sendKey calls on success.
func (s *skyStreamSession) pairAndBind() error {
	tid, err := newUUIDv4()
	if err != nil {
		return err
	}
	controllerNonce, err := newUUIDv4()
	if err != nil {
		return err
	}

	pairReq := skyStreamPairRequest{
		CommandName:     skyStreamCommandPairRequest,
		TID:             tid,
		Name:            "Slinky",
		Manufacturer:    skyStreamManufacturer,
		Model:           skyStreamModel,
		ControllerNonce: controllerNonce,
	}
	if err := s.conn.WriteJSON(pairReq); err != nil {
		return fmt.Errorf("failed to send pair request: %w", err)
	}

	var pairResp skyStreamPairResponse
	if err := s.conn.ReadJSON(&pairResp); err != nil {
		return fmt.Errorf("failed to read pair response: %w", err)
	}
	if !pairResp.Status {
		return fmt.Errorf("Sky Stream box rejected pairing")
	}

	token, err := computeSkyStreamAuthToken(s.certFingerprint, pairResp.PairingCode, controllerNonce, pairResp.STBNonce)
	if err != nil {
		return fmt.Errorf("failed to compute auth token: %w", err)
	}

	bindReq := skyStreamBindRequest{
		CommandName: skyStreamCommandBindRequest,
		TID:         tid,
		AuthToken:   token,
	}
	if err := s.conn.WriteJSON(bindReq); err != nil {
		return fmt.Errorf("failed to send bind request: %w", err)
	}

	var bindResp skyStreamBindResponse
	if err := s.conn.ReadJSON(&bindResp); err != nil {
		return fmt.Errorf("failed to read bind response: %w", err)
	}
	if !bindResp.Status {
		// The protocol doc warns repeated failed binds can lock the box
		// out until reboot - callers should not retry this aggressively.
		return fmt.Errorf("Sky Stream box rejected bind request")
	}

	s.tid = tid
	s.authToken = token
	s.bindID = bindResp.BindID
	return nil
}

// sendKey sends a single key press. pairAndBind must have succeeded first.
func (s *skyStreamSession) sendKey(key string) error {
	req := skyStreamKeyCommandRequest{
		CommandName: skyStreamCommandKeyCommandRequest,
		TID:         s.tid,
		AuthToken:   s.authToken,
		BindID:      s.bindID,
		Cmd:         skyStreamKeyCmd,
		Key:         key,
	}
	if err := s.conn.WriteJSON(req); err != nil {
		return fmt.Errorf("failed to send key command: %w", err)
	}

	var resp skyStreamKeyCommandResponse
	if err := s.conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("failed to read key command response: %w", err)
	}
	if !resp.Status {
		if resp.Error != "" {
			return fmt.Errorf("Sky Stream box rejected key %q: %s", key, resp.Error)
		}
		return fmt.Errorf("Sky Stream box rejected key %q", key)
	}
	return nil
}

// newUUIDv4 generates a random RFC 4122 version 4 UUID string, used for the
// tid and controllernonce fields the protocol requires.
func newUUIDv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// skyStreamState holds the one long-lived session Slinky reuses across
// button presses, since pairing+binding is expensive enough (a real
// round-trip to the box, plus the documented lockout risk on repeated
// failed binds) that it shouldn't happen on every command.
var skyStreamState struct {
	mu      sync.Mutex
	session *skyStreamSession
}

// sendSkyStreamCommand sends command (a Slinky button ID, see
// html/remote.html) to the configured Sky Stream box, establishing and
// caching a session on first use and transparently reconnecting once if
// the cached session's connection has gone bad.
func sendSkyStreamCommand(ctx context.Context, cfg SkyStream, command string) error {
	key, ok := skyStreamKeys[command]
	if !ok {
		return fmt.Errorf("unknown sky stream command: %s", command)
	}

	skyStreamState.mu.Lock()
	defer skyStreamState.mu.Unlock()

	if skyStreamState.session == nil {
		sess, err := connectSkyStream(ctx, cfg)
		if err != nil {
			return err
		}
		skyStreamState.session = sess
	}

	if err := skyStreamState.session.sendKey(key); err != nil {
		skyStreamState.session = nil
		sess, connErr := connectSkyStream(ctx, cfg)
		if connErr != nil {
			return fmt.Errorf("%w (reconnect also failed: %v)", err, connErr)
		}
		if sendErr := sess.sendKey(key); sendErr != nil {
			return sendErr
		}
		skyStreamState.session = sess
	}
	return nil
}

// connectSkyStream wakes the box if configured to (see wakeIfNeeded), loads
// the configured client certificate, dials the box, and pairs+binds a fresh
// session.
func connectSkyStream(ctx context.Context, cfg SkyStream) (*skyStreamSession, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load Sky Stream client certificate: %w", err)
	}

	woke := wakeIfNeeded(cfg)

	u := url.URL{Scheme: "wss", Host: fmt.Sprintf("%s:8091", cfg.Host), Path: "/iptarget"}
	origin := fmt.Sprintf("https://%s:8091/", cfg.Host)
	conn, err := dialSkyStream(ctx, skyStreamDialConfig{URL: u.String(), ServerName: "sky.xcal.tv", Origin: origin, Cert: cert})
	if err != nil {
		return nil, err
	}

	sess, err := newSkyStreamSession(conn, cert)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := sess.pairAndBind(); err != nil {
		conn.Close()
		return nil, err
	}

	if woke {
		// Wake-on-LAN only wakes the box's network stack - the TV app can
		// still be in standby, so nudge it the same way the reference
		// client does.
		time.Sleep(1 * time.Second)
		if err := sess.sendKey(skyStreamKeys["power"]); err != nil {
			log.Printf("Warning: Sky Stream Power nudge after Wake-on-LAN failed: %v\n", err)
		}
	}

	return sess, nil
}
