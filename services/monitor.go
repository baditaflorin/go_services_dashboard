package services

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/baditaflorin/go_services_dashboard/models"
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
	servicesList := make([]*models.Service, 0, len(m.registry.services))
	for _, svc := range m.registry.services {
		servicesList = append(servicesList, svc)
	}
	m.registry.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Limit concurrent checks

	for _, svc := range servicesList {
		wg.Add(1)
		go func(s *models.Service) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			m.checkService(s)
		}(svc)
	}

	wg.Wait()
	log.Printf("Health check completed for %d services", len(servicesList))
}

func (m *Monitor) checkService(svc *models.Service) {
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
		// Accept "healthy" or "ok" as healthy status
		if healthResp.Status == "healthy" || healthResp.Status == "ok" {
			svc.Status = "healthy"
		} else if healthResp.Status != "" {
			svc.Status = "unhealthy"
		} else {
			// No status field but got 200 OK
			svc.Status = "healthy"
		}
	} else {
		// Couldn't parse JSON but got 200 OK - assume healthy
		svc.Status = "healthy"
	}
}

// ValidateTestLinks checks all service test URLs and records HTTP status
func (m *Monitor) ValidateTestLinks() {
	m.registry.mu.RLock()
	servicesList := make([]*models.Service, 0, len(m.registry.services))
	for _, svc := range m.registry.services {
		servicesList = append(servicesList, svc)
	}
	m.registry.mu.RUnlock()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent checks

	for _, svc := range servicesList {
		wg.Add(1)
		go func(s *models.Service) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			m.validateTestLink(s)
		}(svc)
	}

	wg.Wait()
	log.Printf("Test link validation completed for %d services", len(servicesList))
}

func (m *Monitor) validateTestLink(svc *models.Service) {
	if svc.ExampleURL == "" {
		return
	}

	resp, err := m.client.Get(svc.ExampleURL)

	m.registry.mu.Lock()
	defer m.registry.mu.Unlock()

	svc.TestChecked = time.Now()

	if err != nil {
		svc.TestStatus = 0 // Connection error
		return
	}
	defer resp.Body.Close()

	svc.TestStatus = resp.StatusCode
}
