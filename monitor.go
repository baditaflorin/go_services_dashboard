package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// Monitor handles background health checking
type Monitor struct {
	registry *Registry
	client   *http.Client
	interval time.Duration
}

// NewMonitor creates a new health monitor
func NewMonitor(r *Registry) *Monitor {
	return &Monitor{
		registry: r,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		interval: 30 * time.Second,
	}
}

// Start begins the monitoring loop
func (m *Monitor) Start() {
	// Initial check
	m.checkAll()

	ticker := time.NewTicker(m.interval)
	for range ticker.C {
		m.checkAll()
	}
}

func (m *Monitor) checkAll() {
	m.registry.mu.RLock()
	services := make([]*Service, 0, len(m.registry.services))
	for _, svc := range m.registry.services {
		services = append(services, svc)
	}
	m.registry.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Limit concurrent checks

	for _, svc := range services {
		wg.Add(1)
		go func(s *Service) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			m.checkService(s)
		}(svc)
	}

	wg.Wait()
	log.Printf("Health check completed for %d services", len(services))
}

func (m *Monitor) checkService(svc *Service) {
	start := time.Now()

	// Construct Internal Health URL for reliable checking within cluster
	checkURL := svc.HealthURL
	if svc.DockerName != "" && svc.Port > 0 {
		checkURL = fmt.Sprintf("http://%s:%d/health", svc.DockerName, svc.Port)
	}

	// Check Health
	resp, err := m.client.Get(checkURL)
	elapsed := time.Since(start).Milliseconds()

	m.registry.mu.Lock()

	svc.LastChecked = time.Now()
	svc.ResponseMs = elapsed

	// Handle Health Result
	if err != nil {
		svc.Status = "unhealthy"
	} else {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			svc.Status = "unhealthy"
		} else {
			// Try to parse version
			var healthResp struct {
				Status  string `json:"status"`
				Version string `json:"version"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&healthResp); err == nil {
				if healthResp.Version != "" {
					svc.Version = healthResp.Version
				}
				if healthResp.Status == "healthy" {
					svc.Status = "healthy"
				} else {
					svc.Status = "unhealthy"
				}
			} else {
				svc.Status = "healthy" // Assume healthy if 200 OK
			}
		}
	}

	m.registry.mu.Unlock()
}

// TestActiveLink checks the ExampleURL on demand (manual trigger)
func (m *Monitor) TestActiveLink(id string) (string, error) {
	m.registry.mu.RLock()
	svc, exists := m.registry.services[id]
	m.registry.mu.RUnlock()

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

	m.registry.mu.Lock()
	svc.TestStatus = status
	m.registry.mu.Unlock()

	return status, nil
}
