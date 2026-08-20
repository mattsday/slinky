package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withCfg swaps the package-level cfg for the duration of a test and
// restores the previous value afterwards, since the HTTP handlers under
// test all read from the shared global.
func withCfg(t *testing.T, c Config) {
	t.Helper()
	orig := cfg
	cfg = c
	t.Cleanup(func() { cfg = orig })
}

func TestHLSPlaylist(t *testing.T) {
	withCfg(t, Config{
		Stream: Stream{
			HLS: []HLSStream{
				{
					Name:       "1080p",
					Location:   "/stream/0.m3u8",
					Bandwidth:  8000000,
					Resolution: "1920x1080",
					Codecs:     "hvc1.1.6.L120.B0,mp4a.40.2",
					FrameRate:  "25.000",
				},
				{
					Name:       "480p",
					Location:   "/stream/2.m3u8",
					Bandwidth:  1500000,
					Resolution: "854x480",
					Codecs:     "hvc1.1.6.L120.B0,mp4a.40.2",
					FrameRate:  "25.000",
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	rec := httptest.NewRecorder()
	hlsPlaylist(rec, req)

	res := rec.Result()
	if ct := res.Header.Get("Content-Type"); ct != "application/vnd.apple.mpegurl" {
		t.Errorf("Content-Type = %q, want application/vnd.apple.mpegurl", ct)
	}

	body := rec.Body.String()
	wantLines := []string{
		"#EXTM3U",
		"#EXT-X-VERSION:3",
		`#EXT-X-STREAM-INF:BANDWIDTH=8000000,NAME="1080p",RESOLUTION=1920x1080,CODECS="hvc1.1.6.L120.B0,mp4a.40.2",FRAME-RATE=25.000`,
		"/stream/0.m3u8",
		`#EXT-X-STREAM-INF:BANDWIDTH=1500000,NAME="480p",RESOLUTION=854x480,CODECS="hvc1.1.6.L120.B0,mp4a.40.2",FRAME-RATE=25.000`,
		"/stream/2.m3u8",
	}
	for _, want := range wantLines {
		if !strings.Contains(body, want) {
			t.Errorf("playlist body missing line %q\ngot:\n%s", want, body)
		}
	}
}

func TestHLSPlaylist_NoStreamsConfigured(t *testing.T) {
	withCfg(t, Config{})

	req := httptest.NewRequest(http.MethodGet, "/playlist.m3u8", nil)
	rec := httptest.NewRecorder()
	hlsPlaylist(rec, req)

	body := rec.Body.String()
	if body != "#EXTM3U\n#EXT-X-VERSION:3\n" {
		t.Errorf("body = %q, want just the header lines with no streams configured", body)
	}
}

func TestApiCall_HarmonySuccess(t *testing.T) {
	harmony := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Harmony request method = %s, want POST", r.Method)
		}
		if want := "/hubs/kitchen/commands/channel-up"; r.URL.Path != want {
			t.Errorf("Harmony request path = %s, want %s", r.URL.Path, want)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	}))
	defer harmony.Close()

	withCfg(t, Config{
		Control: "harmony",
		HarmonyApi: HarmonyApi{
			Url:        harmony.URL,
			DefaultHub: "kitchen",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/call/channel-up", nil)
	req.SetPathValue("call", "channel-up")
	rec := httptest.NewRecorder()
	apiCall(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestApiCall_HarmonyNonOKMessageIs500(t *testing.T) {
	harmony := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"unrecognised command"}`))
	}))
	defer harmony.Close()

	withCfg(t, Config{
		Control: "harmony",
		HarmonyApi: HarmonyApi{
			Url:        harmony.URL,
			DefaultHub: "kitchen",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/call/bogus", nil)
	req.SetPathValue("call", "bogus")
	rec := httptest.NewRecorder()
	apiCall(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when Harmony doesn't reply with message:ok", rec.Code)
	}
}

func TestApiCall_HarmonyUpstreamDownIs500(t *testing.T) {
	withCfg(t, Config{
		Control: "harmony",
		HarmonyApi: HarmonyApi{
			Url:        "http://127.0.0.1:1", // nothing listens here
			DefaultHub: "kitchen",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/call/channel-up", nil)
	req.SetPathValue("call", "channel-up")
	rec := httptest.NewRecorder()
	apiCall(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 when the Harmony API is unreachable", rec.Code)
	}
}

func TestApiCall_UnknownControlModeIs500(t *testing.T) {
	withCfg(t, Config{Control: "not-a-real-mode"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/call/channel-up", nil)
	req.SetPathValue("call", "channel-up")
	rec := httptest.NewRecorder()
	apiCall(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for an unconfigured/unknown control mode", rec.Code)
	}
}

func TestApiCall_MissingCallIs400(t *testing.T) {
	withCfg(t, Config{Control: "harmony"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/call/", nil)
	req.SetPathValue("call", "")
	rec := httptest.NewRecorder()
	apiCall(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when no call is given", rec.Code)
	}
}

func TestPwStatus_NonHarmonyControlReportsOn(t *testing.T) {
	withCfg(t, Config{Control: "skyq"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/pwr", nil)
	rec := httptest.NewRecorder()
	pwStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"off":false`) {
		t.Errorf("body = %s, want off:false for a non-harmony control mode (power status is meaningless without Harmony)", rec.Body.String())
	}
}
