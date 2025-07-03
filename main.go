package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/spf13/viper"
)

var cfg Config

func loadCfg() (err error) {
	viper.SetConfigName("config")
	viper.AddConfigPath("config")
	viper.AutomaticEnv()
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
	r := mux.NewRouter()
	s := http.StripPrefix("/html/static/", http.FileServer(http.Dir("./html/static/")))
	r.HandleFunc("/", home).Methods(http.MethodGet)
	r.HandleFunc("/video", video).Methods(http.MethodGet)
	r.HandleFunc("/instant.m3u8", instant).Methods(http.MethodGet)
	r.PathPrefix("/html/static/").Handler(s)
	api := r.PathPrefix("/api").Subrouter()
	api.Use(apiHandler)
	api.HandleFunc("/v1/pwr", pwStatus).Methods(http.MethodGet)
	api.HandleFunc("/v1/call/power", togglePower).Methods(http.MethodGet)
	api.HandleFunc("/v1/call/{call}", apiCall).Methods(http.MethodGet)

	if cfg.Dev.Enabled {
		log.Println("Warning: Dev mode enabled")
		proxy, err := NewProxy("/0.mF3u8")
		if err != nil {
			panic(err)
		}
		dev := r.PathPrefix("/").Subrouter()
		dev.HandleFunc("/{.*\\.m3u8}", ProxyRequestHandler(proxy))
		dev.HandleFunc("/{.*\\.ts}", ProxyRequestHandler(proxy))
		dev.HandleFunc("/{.*\\.flv}", ProxyRequestHandler(proxy))
	}

	log.Printf("Startup Complete, listening on port %v\n", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}

func apiHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func home(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("home.html").ParseFiles("html/home.html", "html/remote.html"))
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
	// Get current status
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
	vars := mux.Vars(r)
	if vars["call"] == "" {
		http.Error(w, "Unable to get API call", http.StatusBadRequest)
		log.Printf("No API call found\n")
		return
	}
	u := fmt.Sprintf("%v/hubs/%v/commands/%v", cfg.HarmonyApi.Url, cfg.HarmonyApi.DefaultHub, vars["call"])
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
}

func instant(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/vnd.apple.mpegurl")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("#EXTM3U\n#EXTINF:0,\n3.ts\n"))
}

// NewProxy takes target host and creates a reverse proxy
func NewProxy(path string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(fmt.Sprintf("%v", cfg.Dev.Stream))
	if err != nil {
		return nil, err
	}

	log.Printf("Dev Url: %v\n", url)

	proxy := httputil.NewSingleHostReverseProxy(url)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
	}

	return proxy, nil
}

// ProxyRequestHandler handles the http request using proxy
func ProxyRequestHandler(proxy *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	}
}
