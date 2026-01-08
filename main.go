package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

//go:embed frontend
var frontendFS embed.FS

const version = "1.1.0"

// Service represents a monitored microservice
type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	Port        int       `json:"port"`
	DockerName  string    `json:"docker_name"`
	RepoURL     string    `json:"repo_url"`
	ExampleURL  string    `json:"example_url"`
	HealthURL   string    `json:"health_url"`
	Status      string    `json:"status"`
	Version     string    `json:"version"`
	LastChecked time.Time `json:"last_checked"`
	ResponseMs  int64     `json:"response_ms"`
	Tags        []string  `json:"tags"`
}

// Registry holds all services
type Registry struct {
	services map[string]*Service
	mu       sync.RWMutex
}

func NewRegistry() *Registry {
	r := &Registry{
		services: make(map[string]*Service),
	}
	r.loadServices()
	return r
}

func (r *Registry) loadServices() {
	// TODO: Implement proper scanning of ../../* directories
	// For now, add self for testing
	r.AddService(&Service{
		ID:          "services-dashboard",
		Name:        "services-dashboard",
		Category:    "domains",
		Port:        8131,
		Status:      "unknown",
		HealthURL:   "http://localhost:8131/health",
		Description: "The main dashboard",
	})
}

func (r *Registry) AddService(s *Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[s.ID] = s
}

func (r *Registry) HandleListServices(w http.ResponseWriter, req *http.Request) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*Service, 0, len(r.services))
	for _, s := range r.services {
		list = append(list, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (r *Registry) HandleGetService(w http.ResponseWriter, req *http.Request) {
	// Minimal impl
	w.WriteHeader(http.StatusNotImplemented)
}

func (r *Registry) HandleListCategories(w http.ResponseWriter, req *http.Request) {
	categories := []string{"domains", "security", "recon", "infrastructure", "web_analysis"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

func (r *Registry) HandleStats(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_services": len(r.services),
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "43565"
	}

	// Initialize service registry
	registry := NewRegistry()

	// Start background health monitor
	monitor := NewMonitor(registry)
	go monitor.Start()

	// Setup routes
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/services", registry.HandleListServices)
	mux.HandleFunc("/api/services/", registry.HandleGetService)
	mux.HandleFunc("/api/categories", registry.HandleListCategories)
	mux.HandleFunc("/api/stats", registry.HandleStats)

	// Health and version endpoints
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

	// Serve frontend static files
	frontendContent, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		// Log error but don't crash if frontend missing in dev, but for prod we want it
		log.Printf("Warning: frontend not found in embed: %v", err)
	} else {
		mux.Handle("/", http.FileServer(http.FS(frontendContent)))
	}

	log.Printf("Starting Services Dashboard on port %s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, mux))
}
