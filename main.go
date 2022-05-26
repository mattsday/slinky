package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/spf13/viper"
	"html/template"
	"log"
	"net/http"
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
	return nil
}

func main() {
	err := loadCfg()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
		return
	}

	r := mux.NewRouter()
	r.HandleFunc("/", home).Methods("GET")
	api := r.PathPrefix("/api").Subrouter()
	api.Use(apiHandler)
	api.HandleFunc("/v1/pwr", pwStatus).Methods("GET")
	api.HandleFunc("/v1/call/power", togglePower).Methods("GET")
	api.HandleFunc("/v1/call/{call}", apiCall).Methods("GET")
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
	t := template.Must(template.New("home.html").ParseFiles("html/home.html"))
	err := t.Execute(w, nil)
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
