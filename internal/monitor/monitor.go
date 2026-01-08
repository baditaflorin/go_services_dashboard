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

	// PROPER HEALTH CHECK STRATEGY:
	// 1. Test internal /health endpoint
	// 2. Test ExampleURL (actual service functionality)
	// 3. Compute status: healthy (both OK), degraded (/health OK, ExampleURL fails), unhealthy (health fails)

	healthOK := false
	exampleOK := false
	version := ""
	healthError := ""
	exampleError := ""

	// Build permutation lists for internal health check
	names := []string{}
	if svc.DockerName != "" {
		names = append(names, svc.DockerName)
	}
	if svc.ID != "" && svc.ID != svc.DockerName {
		names = append(names, svc.ID)
		names = append(names, svc.ID+"-app-1")
	}
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

	// STEP 1: Test Internal /health endpoint
	for _, name := range uniqueNames {
		for _, port := range ports {
			internalURL := fmt.Sprintf("http://%s:%d/health", name, port)
			resp, err := m.client.Get(internalURL)
			if err == nil && resp.StatusCode == 200 {
				healthOK = true
				version = parseVersion(resp)
				resp.Body.Close()
				svc.DockerName = name
				svc.Port = port
				goto HealthCheckDone
			}
			if err != nil {
				healthError = fmt.Sprintf("Connection: %v", err)
			} else if resp != nil {
				healthError = fmt.Sprintf("HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
		}
	}

	// Fallback to public HealthURL if internal failed
	if !healthOK && svc.HealthURL != "" {
		resp, err := m.client.Get(svc.HealthURL)
		if err == nil && resp.StatusCode == 200 {
			healthOK = true
			version = parseVersion(resp)
			resp.Body.Close()
		} else {
			if err != nil {
				healthError = fmt.Sprintf("Public health: %v", err)
			} else if resp != nil {
				healthError = fmt.Sprintf("Public health: HTTP %d", resp.StatusCode)
				resp.Body.Close()
			}
		}
	}

HealthCheckDone:

	// STEP 2: Test ExampleURL (actual service functionality)
	if svc.ExampleURL != "" {
		resp, err := m.client.Get(svc.ExampleURL)
		if err == nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				exampleOK = true
			} else {
				exampleError = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			resp.Body.Close()
		} else {
			exampleError = fmt.Sprintf("Connection: %v", err)
		}
	} else {
		// No ExampleURL configured - can't verify actual functionality
		exampleOK = true // Assume OK if not configured
		exampleError = "No ExampleURL configured"
	}

	// STEP 3: Compute final status
	var status string
	var lastError string
	if healthOK && exampleOK {
		status = "healthy"
		lastError = ""
	} else if healthOK && !exampleOK {
		status = "degraded" // /health works but actual service broken
		lastError = exampleError
	} else {
		status = "unhealthy"
		lastError = healthError
	}

	elapsed := time.Since(start).Milliseconds()

	m.registry.Mu.Lock()
	svc.LastChecked = time.Now()
	svc.ResponseMs = elapsed
	svc.Status = status
	svc.HealthStatus = map[bool]string{true: "ok", false: "fail"}[healthOK]
	svc.ExampleStatus = map[bool]string{true: "ok", false: "fail"}[exampleOK]
	svc.LastError = lastError
	if version != "" {
		svc.Version = version
	}
	// Track health history (last 5 checks)
	svc.HealthHistory = append(svc.HealthHistory, status)
	if len(svc.HealthHistory) > 5 {
		svc.HealthHistory = svc.HealthHistory[1:]
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

// TestActiveLink tests if the service's ExampleURL is actually working
// This tests the REAL functionality, not just the /health endpoint
func (m *Monitor) TestActiveLink(id string) (string, string, error) {
	m.registry.Mu.RLock()
	svc, exists := m.registry.Services[id]
	m.registry.Mu.RUnlock()

	if !exists {
		return "", "", fmt.Errorf("service not found")
	}

	client := http.Client{
		Timeout: 10 * time.Second,
	}

	status := "failed"
	errorMsg := ""
	start := time.Now()

	// PRIORITY 1: Test the actual ExampleURL (the service's main functionality)
	if svc.ExampleURL != "" {
		resp, err := client.Get(svc.ExampleURL)
		elapsed := time.Since(start).Milliseconds()

		if err != nil {
			errorMsg = fmt.Sprintf("Connection error: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				status = "passing"
				errorMsg = fmt.Sprintf("HTTP %d in %dms", resp.StatusCode, elapsed)
			} else {
				errorMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}
		}
	} else {
		// FALLBACK: If no ExampleURL, test internal /health endpoint
		names := []string{}
		if svc.DockerName != "" {
			names = append(names, svc.DockerName)
		}
		if svc.ID != "" && svc.ID != svc.DockerName {
			names = append(names, svc.ID+"-app-1")
		}

		ports := []int{}
		if svc.Port > 0 {
			ports = append(ports, svc.Port)
		}
		if svc.Port != 8080 {
			ports = append(ports, 8080)
		}

		for _, name := range names {
			for _, port := range ports {
				testURL := fmt.Sprintf("http://%s:%d/health", name, port)
				resp, err := client.Get(testURL)
				if err == nil {
					defer resp.Body.Close()
					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						status = "passing"
						errorMsg = fmt.Sprintf("Internal health OK (port %d)", port)
						goto TestComplete
					}
				}
			}
		}
		if status == "failed" {
			errorMsg = "No ExampleURL configured, internal health check failed"
		}
	}

TestComplete:
	m.registry.Mu.Lock()
	svc.TestStatus = status
	svc.TestError = errorMsg
	m.registry.Mu.Unlock()

	return status, errorMsg, nil
}
