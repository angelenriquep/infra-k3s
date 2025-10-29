package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type APIRequest struct {
	ID        int       `json:"id"`
	ClientIP  string    `json:"client_ip"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent,omitempty"`
}

type Response struct {
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

var db *sql.DB

var (
	// Métricas de Prometheus
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "endpoint"},
	)

	dbConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Number of active database connections",
		},
	)
)

func init() {
	// Registrar métricas
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(dbConnectionsActive)
}

func main() {
	// Configure PostgreSQL connection
	dbHost := getEnv("DB_HOST", "api-gateway-postgres.default.svc.cluster.local")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "api_gateway_app")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "api_gateway_db")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	if dbPassword == "" {
		log.Fatal("DB_PASSWORD environment variable is required")
	}

	// Database connection
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}
	defer db.Close()

	// Verify connection
	err = db.Ping()
	if err != nil {
		log.Fatal("Error pinging database:", err)
	}

	// Create table if not exists
	createTable()

	// Configurar rutas
	r := mux.NewRouter()

	// Aplicar middleware de métricas
	r.Use(prometheusMiddleware)

	r.HandleFunc("/", handleRoot).Methods("GET")
	r.HandleFunc("/api", handleAPIPost).Methods("POST")
	r.HandleFunc("/api", handleAPIGet).Methods("GET")
	r.HandleFunc("/health", handleHealth).Methods("GET")

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	port := getEnv("PORT", "3000")
	log.Printf("🚀 Backend Go server starting on port %s", port)
	log.Printf("🗄️  Connected to PostgreSQL at %s:%s", dbHost, dbPort)

	log.Fatal(http.ListenAndServe(":"+port, r))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message:   "Hello from Go Backend with PostgreSQL!",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"version":     "1.0.0",
			"environment": getEnv("ENVIRONMENT", "development"),
			"pod_name":    getEnv("HOSTNAME", "unknown"),
			"go_version":  "1.21",
			"database":    "PostgreSQL (Zalando Operator)",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleAPIPost(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	userAgent := r.Header.Get("User-Agent")

	// Insertar en la base de datos
	query := `INSERT INTO api_requests (client_ip, user_agent, timestamp) VALUES ($1, $2, $3) RETURNING id`
	var id int
	err := db.QueryRow(query, clientIP, userAgent, time.Now()).Scan(&id)
	if err != nil {
		log.Printf("Error inserting request: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

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
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	query := `SELECT id, client_ip, user_agent, timestamp FROM api_requests ORDER BY timestamp DESC LIMIT $1`
	rows, err := db.Query(query, limit)
	if err != nil {
		log.Printf("Error querying requests: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var requests []APIRequest
	for rows.Next() {
		var req APIRequest
		err := rows.Scan(&req.ID, &req.ClientIP, &req.UserAgent, &req.Timestamp)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		requests = append(requests, req)
	}

	response := Response{
		Message:   fmt.Sprintf("Retrieved %d API requests", len(requests)),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"requests": requests,
			"count":    len(requests),
			"limit":    limit,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	err := db.Ping()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"database":  "connected",
	})
}

func createTable() {
	query := `
	CREATE TABLE IF NOT EXISTS api_requests (
		id SERIAL PRIMARY KEY,
		client_ip VARCHAR(45) NOT NULL,
		user_agent TEXT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_api_requests_timestamp ON api_requests(timestamp DESC);
	`

	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Error creating table:", err)
	}
	log.Println("✅ Database table ready")
}

func getClientIP(r *http.Request) string {
	// Check proxy headers
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

// Middleware para métricas
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrapper para capturar el status code
		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		// Registrar métricas
		duration := time.Since(start).Seconds()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, fmt.Sprintf("%d", ww.statusCode)).Inc()
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
