package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// generateTestClientCert creates an ephemeral, self-signed EC P-256
// certificate (matching the real protocol's documented client cert type)
// for use as a dialSkyStream client cert in tests. It has no relationship
// to Sky's real embedded certificate - fake box test doubles in this file
// accept any client cert, mirroring how a Sky Stream box itself only
// requires *a* client cert be presented (mTLS), not one from a specific CA.
func generateTestClientCertPEM(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "slinky-test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func generateTestClientCert(t *testing.T) tls.Certificate {
	t.Helper()
	certPEM, keyPEM := generateTestClientCertPEM(t)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("build tls.Certificate: %v", err)
	}
	return cert
}

// generateTestClientCertFiles writes a fresh self-signed test cert/key pair
// to files under t.TempDir(), for tests that go through connectSkyStream
// (which loads the cert from disk via cfg.CertFile/KeyFile) rather than
// constructing a tls.Certificate directly.
func generateTestClientCertFiles(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	certPEM, keyPEM := generateTestClientCertPEM(t)
	dir := t.TempDir()
	certPath = dir + "/cert.pem"
	keyPath = dir + "/key.pem"
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert file: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	return certPath, keyPath
}

// fakeSkyStreamBox is a local test double for a Sky Stream STB: it speaks
// real TLS with mandatory mTLS (any client cert accepted, matching the real
// box's requirement) and upgrades to a real WebSocket at /iptarget, so
// dialSkyStream and skyStreamSession exercise the genuine transport stack
// end-to-end, not just the JSON protocol logic. Per-test behavior is
// supplied via the handler func.
type fakeSkyStreamBox struct {
	srv *httptest.Server
}

func newFakeSkyStreamBox(t *testing.T, handler func(t *testing.T, r *http.Request, conn *websocket.Conn)) *fakeSkyStreamBox {
	t.Helper()

	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc("/iptarget", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("fake box: upgrade failed: %v", err)
			return
		}
		defer conn.Close()
		handler(t, r, conn)
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return &fakeSkyStreamBox{srv: srv}
}

// wsURL returns the wss:// URL for /iptarget on this fake box.
func (f *fakeSkyStreamBox) wsURL() string {
	return "wss" + strings.TrimPrefix(f.srv.URL, "https") + "/iptarget"
}

func dialFakeBox(t *testing.T, box *fakeSkyStreamBox, cert tls.Certificate) *skyStreamSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := dialSkyStream(ctx, skyStreamDialConfig{URL: box.wsURL(), Cert: cert})
	if err != nil {
		t.Fatalf("dialSkyStream: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	sess, err := newSkyStreamSession(conn, cert)
	if err != nil {
		t.Fatalf("newSkyStreamSession: %v", err)
	}
	return sess
}

// TestSkyStreamSession_FullFlow_Success drives a complete pair -> bind ->
// key command sequence against the fake box, with the box independently
// re-deriving the expected auth token from the client certificate it
// actually received over TLS (via r.TLS.PeerCertificates) and the nonces
// exchanged, and rejecting the bind if the client's token doesn't match.
// That's the strongest check available without real hardware: it proves
// the client cert is genuinely presented over mTLS and that the token
// derivation is wired to the real connection, not just internally
// consistent.
func TestSkyStreamSession_FullFlow_Success(t *testing.T) {
	const wantKey = "ArrowUp"
	keyReceived := make(chan string, 1)

	box := newFakeSkyStreamBox(t, func(t *testing.T, r *http.Request, conn *websocket.Conn) {
		if len(r.TLS.PeerCertificates) == 0 {
			t.Error("fake box: no client certificate presented")
			return
		}
		fp, err := skyStreamCertFingerprint(tls.Certificate{Certificate: [][]byte{r.TLS.PeerCertificates[0].Raw}})
		if err != nil {
			t.Errorf("fake box: fingerprint: %v", err)
			return
		}

		var pairReq skyStreamPairRequest
		if err := conn.ReadJSON(&pairReq); err != nil {
			t.Errorf("fake box: read pair request: %v", err)
			return
		}
		if pairReq.CommandName != skyStreamCommandPairRequest {
			t.Errorf("fake box: pair request command_name = %q", pairReq.CommandName)
		}
		if pairReq.Manufacturer != skyStreamManufacturer || pairReq.Model != skyStreamModel {
			t.Errorf("fake box: pair request manufacturer/model = %q/%q, want %q/%q",
				pairReq.Manufacturer, pairReq.Model, skyStreamManufacturer, skyStreamModel)
		}

		const pairingCode = "                 4 2"
		const stbNonce = "fake-stb-nonce"
		if err := conn.WriteJSON(skyStreamPairResponse{
			CommandName: skyStreamCommandPairRequest,
			Name:        "Fake Box",
			PairingCode: pairingCode,
			Status:      true,
			STBNonce:    stbNonce,
			TID:         pairReq.TID,
		}); err != nil {
			t.Errorf("fake box: write pair response: %v", err)
			return
		}

		wantToken, err := computeSkyStreamAuthToken(fp, pairingCode, pairReq.ControllerNonce, stbNonce)
		if err != nil {
			t.Errorf("fake box: compute expected token: %v", err)
			return
		}

		var bindReq skyStreamBindRequest
		if err := conn.ReadJSON(&bindReq); err != nil {
			t.Errorf("fake box: read bind request: %v", err)
			return
		}
		if bindReq.AuthToken != wantToken {
			t.Errorf("fake box: bind auth token = %q, want %q (independently derived from the peer cert)", bindReq.AuthToken, wantToken)
			_ = conn.WriteJSON(skyStreamBindResponse{CommandName: skyStreamCommandBindRequest, Status: false, TID: bindReq.TID})
			return
		}
		const bindID = 7
		if err := conn.WriteJSON(skyStreamBindResponse{
			CommandName: skyStreamCommandBindRequest,
			Status:      true,
			BindID:      bindID,
			TID:         bindReq.TID,
		}); err != nil {
			t.Errorf("fake box: write bind response: %v", err)
			return
		}

		var keyReq skyStreamKeyCommandRequest
		if err := conn.ReadJSON(&keyReq); err != nil {
			t.Errorf("fake box: read key command: %v", err)
			return
		}
		if keyReq.BindID != bindID || keyReq.AuthToken != wantToken || keyReq.Cmd != skyStreamKeyCmd {
			t.Errorf("fake box: key command = %+v, want bind_id=%d authtoken=%q cmd=%q", keyReq, bindID, wantToken, skyStreamKeyCmd)
		}
		keyReceived <- keyReq.Key
		_ = conn.WriteJSON(skyStreamKeyCommandResponse{
			CommandName: skyStreamCommandKeyCommandRequest,
			Status:      true,
			TID:         keyReq.TID,
		})
	})

	cert := generateTestClientCert(t)
	sess := dialFakeBox(t, box, cert)

	if err := sess.pairAndBind(); err != nil {
		t.Fatalf("pairAndBind: %v", err)
	}
	if err := sess.sendKey(wantKey); err != nil {
		t.Fatalf("sendKey: %v", err)
	}

	select {
	case got := <-keyReceived:
		if got != wantKey {
			t.Errorf("box received key %q, want %q", got, wantKey)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fake box never received the key command")
	}
}

func TestSkyStreamSession_PairingRejected(t *testing.T) {
	box := newFakeSkyStreamBox(t, func(t *testing.T, r *http.Request, conn *websocket.Conn) {
		var pairReq skyStreamPairRequest
		if err := conn.ReadJSON(&pairReq); err != nil {
			return
		}
		_ = conn.WriteJSON(skyStreamPairResponse{
			CommandName: skyStreamCommandPairRequest,
			Status:      false,
			TID:         pairReq.TID,
		})
	})

	sess := dialFakeBox(t, box, generateTestClientCert(t))
	if err := sess.pairAndBind(); err == nil {
		t.Fatal("expected an error when the box rejects pairing, got nil")
	}
}

func TestSkyStreamSession_BindRejected(t *testing.T) {
	box := newFakeSkyStreamBox(t, func(t *testing.T, r *http.Request, conn *websocket.Conn) {
		var pairReq skyStreamPairRequest
		if err := conn.ReadJSON(&pairReq); err != nil {
			return
		}
		_ = conn.WriteJSON(skyStreamPairResponse{
			CommandName: skyStreamCommandPairRequest,
			PairingCode: "                 1 1",
			Status:      true,
			STBNonce:    "n",
			TID:         pairReq.TID,
		})

		var bindReq skyStreamBindRequest
		if err := conn.ReadJSON(&bindReq); err != nil {
			return
		}
		// Simulate the box refusing the bind outright (e.g. wrong token),
		// regardless of what was sent.
		_ = conn.WriteJSON(skyStreamBindResponse{
			CommandName: skyStreamCommandBindRequest,
			Status:      false,
			TID:         bindReq.TID,
		})
	})

	sess := dialFakeBox(t, box, generateTestClientCert(t))
	if err := sess.pairAndBind(); err == nil {
		t.Fatal("expected an error when the box rejects the bind request, got nil")
	}
}

func TestSkyStreamSession_KeyCommandRejected(t *testing.T) {
	box := newFakeSkyStreamBox(t, func(t *testing.T, r *http.Request, conn *websocket.Conn) {
		var pairReq skyStreamPairRequest
		if err := conn.ReadJSON(&pairReq); err != nil {
			return
		}
		_ = conn.WriteJSON(skyStreamPairResponse{
			CommandName: skyStreamCommandPairRequest,
			PairingCode: "                 1 1",
			Status:      true,
			STBNonce:    "n",
			TID:         pairReq.TID,
		})

		var bindReq skyStreamBindRequest
		if err := conn.ReadJSON(&bindReq); err != nil {
			return
		}
		_ = conn.WriteJSON(skyStreamBindResponse{
			CommandName: skyStreamCommandBindRequest,
			Status:      true,
			BindID:      1,
			TID:         bindReq.TID,
		})

		var keyReq skyStreamKeyCommandRequest
		if err := conn.ReadJSON(&keyReq); err != nil {
			return
		}
		_ = conn.WriteJSON(skyStreamKeyCommandResponse{
			CommandName: skyStreamCommandKeyCommandRequest,
			Status:      false,
			TID:         keyReq.TID,
			Error:       "unrecognized",
		})
	})

	sess := dialFakeBox(t, box, generateTestClientCert(t))
	if err := sess.pairAndBind(); err != nil {
		t.Fatalf("pairAndBind: %v", err)
	}
	err := sess.sendKey("NotARealKey")
	if err == nil {
		t.Fatal("expected an error for a key the box doesn't recognize, got nil")
	}
	if !strings.Contains(err.Error(), "unrecognized") {
		t.Errorf("error = %v, want it to mention the box's %q error", err, "unrecognized")
	}
}

// TestFakeSkyStreamBox_RejectsConnectionsWithoutClientCert confirms the
// test double actually enforces mTLS the way a real Sky Stream box does -
// otherwise the "success" tests above would be meaningless as transport
// coverage.
func TestFakeSkyStreamBox_RejectsConnectionsWithoutClientCert(t *testing.T) {
	box := newFakeSkyStreamBox(t, func(t *testing.T, r *http.Request, conn *websocket.Conn) {
		t.Error("fake box: handler should not run for a connection with no client cert")
	})

	conn, dialErr := tls.Dial("tcp", strings.TrimPrefix(box.srv.URL, "https://"), &tls.Config{InsecureSkipVerify: true})
	if dialErr == nil {
		defer conn.Close()
		// The client side of a Go TLS handshake can return from Dial
		// before the server's certificate-requirement rejection has been
		// processed; forcing a round trip surfaces it.
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		_, dialErr = conn.Write([]byte{0})
		if dialErr == nil {
			_, dialErr = conn.Read(make([]byte, 1))
		}
	}
	if dialErr == nil {
		t.Fatal("expected the connection to fail without a client certificate, but it succeeded")
	}
}

func TestNewUUIDv4_LooksLikeAUUID(t *testing.T) {
	id, err := newUUIDv4()
	if err != nil {
		t.Fatalf("newUUIDv4: %v", err)
	}
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("uuid = %q, want 5 hyphen-separated groups", id)
	}
	lengths := []int{8, 4, 4, 4, 12}
	for i, want := range lengths {
		if len(parts[i]) != want {
			t.Errorf("uuid group %d = %q (len %d), want len %d", i, parts[i], len(parts[i]), want)
		}
	}
	if parts[2][0] != '4' {
		t.Errorf("uuid version nibble = %q, want '4'", parts[2][0:1])
	}
	variantNibble, err := strconv.ParseInt(parts[3][0:1], 16, 8)
	if err != nil {
		t.Fatalf("parse variant nibble: %v", err)
	}
	if variantNibble < 8 || variantNibble > 0xb {
		t.Errorf("uuid variant nibble = %x, want one of 8/9/a/b", variantNibble)
	}
}

func TestSendSkyStreamCommand_UnknownCommandErrors(t *testing.T) {
	err := sendSkyStreamCommand(context.Background(), SkyStream{}, "not-a-real-command")
	if err == nil {
		t.Fatal("expected an error for an unknown Sky Stream command, got nil")
	}
}

// TestConnectSkyStream_FailedDialAfterWoLHintsAtHostNetworking guards
// against a real deployment failure mode hit this session: in Docker, a
// Wake-on-LAN broadcast never crosses a bridge network's NAT boundary, so
// the box never wakes and the subsequent dial fails with a bare "no route
// to host" - which gives no clue that network_mode: host is the fix. The
// error connectSkyStream returns after a WoL attempt must mention it.
func TestConnectSkyStream_FailedDialAfterWoLHintsAtHostNetworking(t *testing.T) {
	origReachable := skyStreamReachable
	origInterval := skyStreamWoLPollInterval
	origMaxPolls := skyStreamWoLMaxPolls
	skyStreamReachable = func(addr string) bool { return false } // never wakes
	skyStreamWoLPollInterval = time.Millisecond
	skyStreamWoLMaxPolls = 2
	defer func() {
		skyStreamReachable = origReachable
		skyStreamWoLPollInterval = origInterval
		skyStreamWoLMaxPolls = origMaxPolls
	}()

	certFile, keyFile := generateTestClientCertFiles(t)
	// Port 0 on loopback: nothing is listening, so the dial fails fast and
	// deterministically instead of depending on real network timeouts.
	cfg := SkyStream{Host: "127.0.0.1", MAC: "04:b8:6a:f2:e7:4c", CertFile: certFile, KeyFile: keyFile}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := connectSkyStream(ctx, cfg)
	if err == nil {
		t.Fatal("expected an error dialing a box that never came up, got nil")
	}
	if !strings.Contains(err.Error(), "network_mode: host") {
		t.Errorf("error = %v, want it to mention network_mode: host after a WoL attempt", err)
	}
}

// TestConnectSkyStream_FailedDialWithoutWoLHasNoHint confirms the hint only
// appears when Wake-on-LAN was actually attempted - otherwise it would be
// misleading noise on a plain unreachable-box error.
func TestConnectSkyStream_FailedDialWithoutWoLHasNoHint(t *testing.T) {
	certFile, keyFile := generateTestClientCertFiles(t)
	cfg := SkyStream{Host: "127.0.0.1", CertFile: certFile, KeyFile: keyFile} // no MAC set

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := connectSkyStream(ctx, cfg)
	if err == nil {
		t.Fatal("expected an error dialing a box that isn't listening, got nil")
	}
	if strings.Contains(err.Error(), "network_mode: host") {
		t.Errorf("error = %v, should not mention Wake-on-LAN/host networking when no MAC was configured", err)
	}
}
