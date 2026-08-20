package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var cfg Config

func loadCfg() (err error) {
	cfg, err = loadConfig("config")
	return err
}

// loadConfig builds the layered config from configDir/config.yaml, merging
// configDir/config-dev.yaml on top if dev.enabled is set, then merging
// $CONFIG_FILE on top of that if present. It uses its own viper instance so
// it can be called repeatedly (e.g. from tests) without leaking state
// between calls.
func loadConfig(configDir string) (result Config, err error) {
	v := viper.New()
	v.SetConfigName("config")
	v.AddConfigPath(configDir)
	v.AutomaticEnv()
	v.SetDefault("control", "harmony") // Set default config to harmy for backwards compatibility
	if err = v.ReadInConfig(); err != nil {
		return result, err
	}
	if err := v.Unmarshal(&result); err != nil {
		log.Printf("Error loading config: %v", err)
		return result, err
	}
	if result.Dev.Enabled {
		v.SetConfigName("config-dev")
		v.AddConfigPath(configDir)
		if err = v.MergeInConfig(); err != nil {
			log.Printf("Warning: %v\n", err)
		}
		if err := v.Unmarshal(&result); err != nil {
			log.Printf("Error loading config: %v", err)
			return result, err
		}
	}
	// Read a config file from environment if present
	if os.Getenv("CONFIG_FILE") != "" {
		log.Printf("Adding config file %v\n", os.Getenv("CONFIG_FILE"))
		v.SetConfigFile(os.Getenv("CONFIG_FILE"))
		if err = v.MergeInConfig(); err != nil {
			log.Printf("Warning: %v\n", err)
		}
		if err := v.Unmarshal(&result); err != nil {
			log.Printf("Error loading config: %v", err)
			return result, err
		}
	}
	if !validControlModes[result.Control] {
		return result, fmt.Errorf("unknown control mode %q (want one of: harmony, skyq)", result.Control)
	}

	return result, nil
}

// validControlModes are the control backends apiCall knows how to dispatch
// to. Checked at startup so a config typo fails fast instead of only
// surfacing as a 500 on the first button press.
var validControlModes = map[string]bool{
	"harmony":   true,
	"skyq":      true,
	"skystream": true,
}

func main() {
	err := loadCfg()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
		return
	}

	log.Printf("Using control mode: %v\n", cfg.Control)

	mux := http.NewServeMux()

	// 1. Wrap the static file handler in a method to resolve the conflict.
	staticServer := http.StripPrefix("/html/static/", http.FileServer(http.Dir("./html/static/")))
	mux.HandleFunc("GET /html/static/", staticServer.ServeHTTP)

	// Page handlers
	mux.HandleFunc("GET /video", video)
	mux.HandleFunc("GET /remote", remote)
	mux.HandleFunc("GET /instant.m3u8", instant)
	mux.HandleFunc("GET /playlist.m3u8", hlsPlaylist)

	// API handlers
	mux.HandleFunc("GET /api/v1/pwr", apiHandler(pwStatus))
	mux.HandleFunc("GET /api/v1/call/power", apiHandler(togglePower))
	mux.HandleFunc("GET /api/v1/call/{call}", apiHandler(apiCall))

	// Dev mode proxy
	if cfg.Dev.Enabled {
		log.Println("Warning: Dev mode enabled")
		proxy, err := NewProxy()
		if err != nil {
			panic(err)
		}
		// This handler will catch all requests not already matched and check if they should be proxied.
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// Check if it's a file to proxy
			if strings.HasSuffix(path, ".m3u8") || strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".flv") {
				ProxyRequestHandler(proxy)(w, r)
				return
			}
			// If not a proxy request, it must be the home page (for GET /).
			if path == "/" && r.Method == http.MethodGet {
				home(w, r)
				return
			}
			// Otherwise, it's a 404
			http.NotFound(w, r)
		})
	} else {
		// If not in dev mode, just handle the root.
		mux.HandleFunc("GET /", home)
	}

	log.Printf("Startup Complete, listening on port %v\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

func hlsPlaylist(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "#EXTM3U")
	fmt.Fprintln(w, "#EXT-X-VERSION:3")
	for _, quality := range cfg.Stream.HLS {
		fmt.Fprintf(w, "#EXT-X-STREAM-INF:BANDWIDTH=%d,NAME=\"%s\",RESOLUTION=%s,CODECS=\"%s\",FRAME-RATE=%s\n", quality.Bandwidth, quality.Name, quality.Resolution, quality.Codecs, quality.FrameRate)
		fmt.Fprintln(w, quality.Location)
	}
}

func apiHandler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	}
}

func home(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("video.html").ParseFiles("html/video.html", "html/remote.html"))
	err := t.Execute(w, cfg)
	if err != nil {
		log.Printf("Error rendering page: %v\n", err)
		return
	}
}

func video(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("video.html").ParseFiles("html/video.html", "html/remote.html"))
	err := t.Execute(w, cfg)
	if err != nil {
		log.Printf("Error rendering page: %v\n", err)
		return
	}
}

func remote(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("remote-home.html").ParseFiles("html/remote-home.html", "html/remote.html"))
	err := t.Execute(w, cfg)
	if err != nil {
		log.Printf("Error rendering page: %v\n", err)
		return
	}
}

func pwStatus(w http.ResponseWriter, r *http.Request) {
	status, err := powerStatus(r.Context())
	if err != nil {
		http.Error(w, "Unable to get status", http.StatusInternalServerError)
		log.Printf("Error getting status: %v\n", err)
		return
	}
	_ = json.NewEncoder(w).Encode(status)
}

func togglePower(w http.ResponseWriter, r *http.Request) {
	status, err := powerStatus(r.Context())
	if err != nil {
		http.Error(w, "Unable to get status", http.StatusInternalServerError)
		log.Printf("Error getting status: %v\n", err)
		return
	}
	var response PowerStatus
	if status.Off || !status.ExpectedActivity {
		response, err = turnOn(r.Context())
		if err != nil {
			http.Error(w, "Unable to turn off", http.StatusInternalServerError)
			log.Printf("Error getting status: %v\n", err)
			return
		}
	} else {
		response, err = turnOff(r.Context())
		if err != nil {
			http.Error(w, "Unable to turn off", http.StatusInternalServerError)
			log.Printf("Error getting status: %v\n", err)
			return
		}
	}
	_ = json.NewEncoder(w).Encode(response)
}

func apiCall(w http.ResponseWriter, r *http.Request) {
	call := r.PathValue("call")
	if call == "" {
		http.Error(w, "Unable to get API call", http.StatusBadRequest)
		log.Printf("No API call found\n")
		return
	}
	switch cfg.Control {
	case "skyq":
		// Use the new Sky Q function
		err := sendSkyCommand(cfg.SkyQ.Host, cfg.SkyQ.Port, call)
		if err != nil {
			http.Error(w, "Unable to send Sky Q command", http.StatusInternalServerError)
			log.Printf("Error sending Sky Q command: %v\n", err)
			return
		}
		// Return a simple success response
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message": "ok"}`)
	case "skystream":
		err := sendSkyStreamCommand(r.Context(), cfg.SkyStream, call)
		if err != nil {
			http.Error(w, "Unable to send Sky Stream command", http.StatusInternalServerError)
			log.Printf("Error sending Sky Stream command: %v\n", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message": "ok"}`)
	case "harmony":
		u := fmt.Sprintf("%v/hubs/%v/commands/%v", cfg.HarmonyApi.Url, cfg.HarmonyApi.DefaultHub, call)
		data, err := request(r.Context(), http.MethodPost, u)
		if err != nil {
			http.Error(w, "Unable to issue command", http.StatusInternalServerError)
			log.Printf("Error sending command: %v\n", err)
			return
		}
		var status RequestResponse
		err = json.Unmarshal(data, &status)
		if err != nil {
			http.Error(w, "Unable to issue command", http.StatusInternalServerError)
			log.Printf("Error sending command: %v\n", err)
			return
		}
		if status.Message != "ok" {
			msg := fmt.Sprintf("API did not come back OK, returned: %v\n", status.Message)
			http.Error(w, msg, http.StatusInternalServerError)
			log.Print(msg)
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	default:
		http.Error(w, "No control method configured", http.StatusInternalServerError)
		log.Println("No control method configured")
	}
}

func instant(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("#EXTM3U\n#EXTINF:0,\n3.ts\n"))
}

func NewProxy() (*httputil.ReverseProxy, error) {
	targetUrl, err := url.Parse(cfg.Dev.Stream)
	if err != nil {
		return nil, err
	}
	log.Printf("Dev proxy enabled for URL: %v\n", targetUrl)
	return httputil.NewSingleHostReverseProxy(targetUrl), nil
}

func ProxyRequestHandler(proxy *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}
