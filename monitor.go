package main

import (
	"encoding/json"
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

	resp, err := m.client.Get(svc.HealthURL)
	elapsed := time.Since(start).Milliseconds()

	m.registry.mu.Lock()
	defer m.registry.mu.Unlock()

	svc.LastChecked = time.Now()
	svc.ResponseMs = elapsed

	if err != nil {
		svc.Status = "unhealthy"
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		svc.Status = "unhealthy"
		return
	}

	// Try to parse version from health response
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
