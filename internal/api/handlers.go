package api

import (
	"encoding/json"
	"net/http"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
	"github.com/baditaflorin/go_services_dashboard/internal/monitor"
)

type Handler struct {
	Registry *models.Registry
	Monitor  *monitor.Monitor
}

func NewHandler(r *models.Registry, m *monitor.Monitor) *Handler {
	return &Handler{
		Registry: r,
		Monitor:  m,
	}
}

func (h *Handler) HandleListServices(w http.ResponseWriter, req *http.Request) {
	list := h.Registry.GetAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func (h *Handler) HandleStats(w http.ResponseWriter, req *http.Request) {
	list := h.Registry.GetAll()
	total := len(list)
	healthy := 0
	unhealthy := 0

	for _, s := range list {
		if s.Status == "healthy" {
			healthy++
		} else if s.Status == "unhealthy" {
			unhealthy++
		}
	}

	healthyPercent := 0.0
	if total > 0 {
		healthyPercent = (float64(healthy) / float64(total)) * 100
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":           total,
		"healthy":         healthy,
		"unhealthy":       unhealthy,
		"healthy_percent": healthyPercent,
	})
}

func (h *Handler) HandleCategories(w http.ResponseWriter, req *http.Request) {
	categories := []string{"domains", "security", "recon", "infrastructure", "web_analysis"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}

func (h *Handler) HandleManualTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/api/test/"):]
	if id == "" {
		http.Error(w, "Missing service ID", http.StatusBadRequest)
		return
	}

	status, err := h.Monitor.TestActiveLink(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":          id,
		"test_status": status,
	})
}

func (h *Handler) HandleCategoryTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	category := r.URL.Path[len("/api/test-category/"):]
	if category == "" {
		http.Error(w, "Missing category", http.StatusBadRequest)
		return
	}

	services := h.Registry.GetAll()
	tested := 0
	passed := 0

	for _, svc := range services {
		if svc.Category == category {
			status, err := h.Monitor.TestActiveLink(svc.ID)
			if err == nil {
				tested++
				if status == "passing" {
					passed++
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"category": category,
		"tested":   tested,
		"passed":   passed,
	})
}
