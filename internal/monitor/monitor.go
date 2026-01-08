package monitor

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// Monitor handles background health checking
type Monitor struct {
	registry *models.Registry
	client   *http.Client
	interval time.Duration
}

// NewMonitor creates a new health monitor
func NewMonitor(r *models.Registry) *Monitor {
	return &Monitor{
		registry: r,
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil // Follow redirects
			},
		},
		interval: 30 * time.Second,
	}
}

// Start begins the monitoring loop
func (m *Monitor) Start() {
	// Initial check
	m.CheckAll()

	ticker := time.NewTicker(m.interval)
	for range ticker.C {
		m.CheckAll()
	}
}

func (m *Monitor) CheckAll() {
	services := m.registry.GetAll()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Limit concurrent checks

	for _, svc := range services {
		wg.Add(1)
		go func(s *models.Service) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			m.CheckService(s)
		}(svc)
	}

	wg.Wait()
	log.Printf("Health check completed for %d services", len(services))
}

func (m *Monitor) CheckService(svc *models.Service) {
	start := time.Now()

	// SMART HYBRID CHECK STRATEGY
	// 1. Internal Permutations (Name x Port)
	// 2. Public Fallback

	status := "unhealthy"
	version := ""

	// Build permutation lists
	names := []string{}
	if svc.DockerName != "" {
		names = append(names, svc.DockerName)
	}
	if svc.ID != "" && svc.ID != svc.DockerName {
		names = append(names, svc.ID)
		names = append(names, svc.ID+"-app-1")
	}
	// unique names
	uniqueNames := make([]string, 0, len(names))
	seenNames := make(map[string]bool)
	for _, n := range names {
		if !seenNames[n] && n != "" {
			uniqueNames = append(uniqueNames, n)
			seenNames[n] = true
		}
	}

	ports := []int{}
	if svc.Port > 0 {
		ports = append(ports, svc.Port)
	}
	if svc.Port != 8080 {
		ports = append(ports, 8080)
	}

	// 1. Try Internal Permutations
	for _, name := range uniqueNames {
		for _, port := range ports {
			internalURL := fmt.Sprintf("http://%s:%d/health", name, port)
			resp, err := m.client.Get(internalURL)
			if err == nil && resp.StatusCode == 200 {
				status = "healthy"
				version = parseVersion(resp)
				resp.Body.Close()
				// Update service with CORRECT found values to speed up next time?
				// Maybe not safe to modify config data in memory permanently if it drifts from source of truth,
				// but for runtime it's fine.
				svc.DockerName = name
				svc.Port = port
				goto CheckComplete
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

CheckComplete:
	// 2. Fallback to Public Check if Internal failed
	if status != "healthy" && svc.HealthURL != "" {
		resp, err := m.client.Get(svc.HealthURL)
		if err == nil && resp.StatusCode == 200 {
			status = "healthy"
			if version == "" {
				version = parseVersion(resp)
			}
			resp.Body.Close()
		} else {
			if resp != nil {
				resp.Body.Close()
			}
		}
	}

	elapsed := time.Since(start).Milliseconds()

	m.registry.Mu.Lock()
	svc.LastChecked = time.Now()
	svc.ResponseMs = elapsed
	svc.Status = status
	if version != "" {
		svc.Version = version
	}
	m.registry.Mu.Unlock()
}

func parseVersion(resp *http.Response) string {
	var healthResp struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&healthResp); err == nil {
		return healthResp.Version
	}
	return ""
}

// TestActiveLink checks the ExampleURL on demand (manual trigger)
func (m *Monitor) TestActiveLink(id string) (string, error) {
	m.registry.Mu.RLock()
	svc, exists := m.registry.Services[id]
	m.registry.Mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("service not found")
	}

	if svc.ExampleURL == "" {
		return "skipped", nil
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(svc.ExampleURL)

	status := "failed"
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			status = "passing"
		}
	}

	m.registry.Mu.Lock()
	svc.TestStatus = status
	m.registry.Mu.Unlock()

	return status, nil
}
