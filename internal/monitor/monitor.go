package monitor

import (
	"encoding/json"
	"fmt"
	"io"
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
	var exampleStatusCode int
	if svc.ExampleURL != "" {
		resp, err := m.client.Get(svc.ExampleURL)
		if err == nil {
			exampleStatusCode = resp.StatusCode
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
	// FIXED LOGIC:
	// - 5xx errors (502, 500, 503) = UNHEALTHY (service is down)
	// - Connection errors (status 0) = UNHEALTHY (can't reach service)
	// - 4xx errors (404, 401) = DEGRADED (service running but endpoint issue)
	// - healthOK && exampleOK = HEALTHY
	// - !healthOK = UNHEALTHY
	var status string
	var lastError string
	if healthOK && exampleOK {
		status = "healthy"
		lastError = ""
	} else if !healthOK {
		status = "unhealthy"
		lastError = healthError
	} else if exampleStatusCode >= 500 || exampleStatusCode == 0 {
		// 5xx errors or connection failures = service is actually DOWN
		status = "unhealthy"
		lastError = exampleError
	} else {
		// 4xx errors = degraded (service runs but has endpoint issues)
		status = "degraded"
		lastError = exampleError
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
	errMsg := ""
	start := time.Now()

	// Build internal test URLs
	// Services run on Docker containers accessible via container names
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

	// PRIORITY 1: Test via internal Docker endpoint with actual API path
	// Example: http://go_dtss-app-1:8080/t/default_token/?url=https://example.com
	if svc.ExampleURL != "" {
		// Extract path from ExampleURL (e.g., /t/default_token/?url=https://example.com)
		pathStart := 0
		for i, c := range svc.ExampleURL {
			if c == '/' && i > 8 { // Skip https://
				pathStart = i
				break
			}
		}
		path := svc.ExampleURL[pathStart:]

		for _, name := range names {
			for _, port := range ports {
				testURL := fmt.Sprintf("http://%s:%d%s", name, port, path)
				resp, err := client.Get(testURL)
				if err == nil {
					defer resp.Body.Close()
					elapsed := time.Since(start).Milliseconds()

					// Check if response is valid JSON (actual service response)
					bodyBytes, _ := io.ReadAll(resp.Body)
					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						// Check if it's valid JSON with expected fields
						var jsonCheck map[string]interface{}
						if json.Unmarshal(bodyBytes, &jsonCheck) == nil {
							// Valid JSON response
							if _, hasResult := jsonCheck["result"]; hasResult {
								status = "passing"
								errMsg = fmt.Sprintf("OK in %dms", elapsed)
								goto TestComplete
							} else if _, hasTool := jsonCheck["tool"]; hasTool {
								status = "passing"
								errMsg = fmt.Sprintf("OK in %dms", elapsed)
								goto TestComplete
							}
						}
						// Valid HTTP but not expected JSON
						status = "passing"
						errMsg = fmt.Sprintf("HTTP %d in %dms", resp.StatusCode, elapsed)
						goto TestComplete
					} else {
						errMsg = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
					}
				}
			}
		}
		// NOTE: If ExampleURL is present but all internal attempts failed, DO NOT FALLBACK to /health
		// The user expects the "Run Test" button to test the ACTUAL service functionality.
		// A 404 or connection error on the ExampleURL should be a failure, even if /health is fine.
		if status == "failed" && errMsg == "" {
			errMsg = "Internal test failed: could not reach service via internal network"
		}
		goto TestComplete
	}

	// FALLBACK: If no ExampleURL, test internal /health endpoint
	for _, name := range names {
		for _, port := range ports {
			testURL := fmt.Sprintf("http://%s:%d/health", name, port)
			resp, err := client.Get(testURL)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					status = "passing"
					errMsg = fmt.Sprintf("Health OK (port %d)", port)
					goto TestComplete
				}
			}
		}
	}

	if status == "failed" && errMsg == "" {
		errMsg = "No ExampleURL configured, internal health check failed"
	}

TestComplete:
	m.registry.Mu.Lock()
	svc.TestStatus = status
	svc.TestError = errMsg
	m.registry.Mu.Unlock()

	return status, errMsg, nil
}
