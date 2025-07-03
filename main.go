package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/spf13/viper"
)

var cfg Config

func loadCfg() (err error) {
	viper.SetConfigName("config")
	viper.AddConfigPath("config")
	viper.AutomaticEnv()
	viper.SetDefault("control", "harmony") // Set default config to harmy for backwards compatibility
	if err = viper.ReadInConfig(); err != nil {
		return err
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("Error loading config: %v", err)
		return err
	}
	if cfg.Dev.Enabled {
		viper.SetConfigName("config-dev")
		viper.AddConfigPath("config")
		if err = viper.MergeInConfig(); err != nil {
			log.Printf("Warning: %v\n", err)
		}
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Printf("Error loading config: %v", err)
			return err
		}
	}
	return nil
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
			log.Printf(msg, status.Message)
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
