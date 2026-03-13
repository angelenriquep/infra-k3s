package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type Response struct {
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// In-memory request store
var (
	requests []map[string]interface{}
	mu       sync.RWMutex
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", handleRoot).Methods("GET")
	r.HandleFunc("/api", handleAPIPost).Methods("POST")
	r.HandleFunc("/api", handleAPIGet).Methods("GET")
	r.HandleFunc("/health", handleHealth).Methods("GET")

	port := getEnv("PORT", "3000")
	log.Printf("🚀 Backend Go server starting on port %s", port)

	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message:   "Hello from Go Backend!",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"version":     "2.0.0",
			"environment": getEnv("ENVIRONMENT", "development"),
			"pod_name":    getEnv("HOSTNAME", "unknown"),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleAPIPost(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	entry := map[string]interface{}{
		"client_ip":  clientIP,
		"user_agent": userAgent,
		"timestamp":  time.Now(),
	}

	mu.Lock()
	requests = append(requests, entry)
	id := len(requests)
	mu.Unlock()

	response := Response{
		Message:   "Request logged successfully",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"id":         id,
			"client_ip":  clientIP,
			"user_agent": userAgent,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func handleAPIGet(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	count := len(requests)
	// Return last 20 max
	start := 0
	if count > 20 {
		start = count - 20
	}
	result := requests[start:]
	mu.RUnlock()

	response := Response{
		Message:   "Retrieved API requests",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"requests": result,
			"count":    len(result),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
	})
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
