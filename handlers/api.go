package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/baditaflorin/go_services_dashboard/services"
)

type API struct {
	registry *services.Registry
	monitor  *services.Monitor
}

func NewAPI(registry *services.Registry, monitor *services.Monitor) *API {
	return &API{registry: registry, monitor: monitor}
}

func (api *API) HandleListServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	services := api.registry.GetAll()
	json.NewEncoder(w).Encode(services)
}

func (api *API) HandleGetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	services := api.registry.GetAll()

	total := len(services)
	healthy := 0
	unhealthy := 0
	unknown := 0
	categories := make(map[string]int)

	for _, s := range services {
		switch s.Status {
		case "healthy":
			healthy++
		case "unhealthy":
			unhealthy++
		default:
			unknown++
		}
		categories[s.Category]++
	}

	var uptime float64
	if total > 0 {
		uptime = float64(healthy) / float64(total) * 100
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":      total,
		"healthy":    healthy,
		"unhealthy":  unhealthy,
		"unknown":    unknown,
		"uptime":     uptime,
		"categories": categories,
	})
}

func (api *API) HandleListCategories(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"domains", "security", "recon", "infrastructure", "web_analysis"})
}

func (api *API) HandleValidateTests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Run validation in background
	go api.monitor.ValidateTestLinks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "started",
		"message": "Test link validation started in background",
	})
}
