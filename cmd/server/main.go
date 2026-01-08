package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/baditaflorin/go_services_dashboard/internal/api"
	"github.com/baditaflorin/go_services_dashboard/internal/config"
	"github.com/baditaflorin/go_services_dashboard/internal/models"
	"github.com/baditaflorin/go_services_dashboard/internal/monitor"
)

const version = "1.8.2"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "43565"
	}

	// 1. Initialize Registry
	registry := models.NewRegistry()

	// 2. Load Services
	config.LoadServices(registry)

	// 3. Start Monitor (Hybrid: Internal -> Public)
	mon := monitor.NewMonitor(registry)
	go mon.Start()

	// 4. Initialize Handlers
	handler := api.NewHandler(registry, mon)

	// 5. Setup Routes
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/services", handler.HandleListServices)
	mux.HandleFunc("/api/stats", handler.HandleStats)
	mux.HandleFunc("/api/categories", handler.HandleCategories)
	mux.HandleFunc("/api/test/", handler.HandleManualTest)
	mux.HandleFunc("/api/test-category/", handler.HandleCategoryTest)
	mux.HandleFunc("/api/events", handler.HandleEvents)
	mux.HandleFunc("/api/refresh", handler.HandleRefresh)

	// System Health
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "services-dashboard",
			"version": version,
		})
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"service": "services-dashboard",
			"version": version,
			"built":   "2026-01-08",
		})
	})

	// Static Assets
	// Serve from ./frontend which Dockerfile copies to /app/frontend
	fs := http.FileServer(http.Dir("./frontend"))
	mux.Handle("/", fs)

	log.Printf("Starting Services Dashboard v%s on port %s", version, port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
