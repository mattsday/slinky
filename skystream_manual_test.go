package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestManualSkyStreamProbe is not part of the automated suite - it's
// skipped unless SKYSTREAM_PROBE_HOST is set, and it dials a real Sky
// Stream box. Useful for repeating Stage 4 hardware verification (see
// docs/plans/sky-stream-and-resilience.md) without a bespoke script.
//
// Usage:
//
//	SKYSTREAM_PROBE_HOST=10.86.0.106 \
//	SKYSTREAM_PROBE_CERT=certs/sky.xcal.tv-ComcastRDKD2DECCICA1-20260430-20270430.pem \
//	SKYSTREAM_PROBE_KEY=certs/softremote_sky_key.pem \
//	go test -run TestManualSkyStreamProbe -v
//
// Set SKYSTREAM_PROBE_MAC to also exercise the Wake-on-LAN path
// (wakeIfNeeded) if the box is unreachable - this raises the test timeout
// need accordingly (see -timeout on the go test invocation).
//
// Set SKYSTREAM_PROBE_KEY_NAME to also send one key command after a
// successful bind - left unset by default, since pairing+binding alone
// carries the documented lockout risk on repeated *failures*, while
// sending a key command visibly acts on the real TV and shouldn't happen
// without deliberately opting in.
func TestManualSkyStreamProbe(t *testing.T) {
	host := os.Getenv("SKYSTREAM_PROBE_HOST")
	if host == "" {
		t.Skip("set SKYSTREAM_PROBE_HOST to run this against a real Sky Stream box")
	}
	certFile := os.Getenv("SKYSTREAM_PROBE_CERT")
	keyFile := os.Getenv("SKYSTREAM_PROBE_KEY")
	if certFile == "" || keyFile == "" {
		t.Fatal("SKYSTREAM_PROBE_CERT and SKYSTREAM_PROBE_KEY must both be set")
	}

	cfg := SkyStream{Host: host, CertFile: certFile, KeyFile: keyFile, MAC: os.Getenv("SKYSTREAM_PROBE_MAC")}
	if cfg.MAC != "" {
		t.Logf("MAC %s configured - will send Wake-on-LAN if %s:8091 isn't already reachable", cfg.MAC, host)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	sess, err := connectSkyStream(ctx, cfg)
	if err != nil {
		t.Fatalf("connectSkyStream (dial + pair + bind): %v", err)
	}
	t.Logf("paired and bound successfully: bind_id=%d tid=%s", sess.bindID, sess.tid)

	if key := os.Getenv("SKYSTREAM_PROBE_KEY_NAME"); key != "" {
		if err := sess.sendKey(key); err != nil {
			t.Fatalf("sendKey(%q): %v", key, err)
		}
		t.Logf("sent key %q successfully", key)
	} else {
		t.Log("SKYSTREAM_PROBE_KEY_NAME not set - stopping after bind, no key command sent")
	}
}
