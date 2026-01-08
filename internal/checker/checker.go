package checker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// CheckServiceResult allows returning results without side effects
type CheckServiceResult struct {
	Status        string
	HealthStatus  string
	ExampleStatus string
	LastError     string
	Version       string
	ResponseMs    int64
}

// CheckService performs the health check logic
func CheckService(client *http.Client, svc *models.Service) CheckServiceResult {
	start := time.Now()

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
			resp, err := client.Get(internalURL)
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
		resp, err := client.Get(svc.HealthURL)
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
		resp, err := client.Get(svc.ExampleURL)
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
		// No ExampleURL configured
		exampleOK = true
		exampleError = "No ExampleURL configured"
	}

	// STEP 3: Compute final status
	var status string
	var lastError string
	if healthOK && exampleOK {
		status = "healthy"
		lastError = ""
	} else if !healthOK {
		status = "unhealthy"
		lastError = healthError
	} else if exampleStatusCode >= 500 || exampleStatusCode == 0 {
		status = "unhealthy"
		lastError = exampleError
	} else {
		status = "degraded"
		lastError = exampleError
	}

	elapsed := time.Since(start).Milliseconds()

	return CheckServiceResult{
		Status:        status,
		HealthStatus:  map[bool]string{true: "ok", false: "fail"}[healthOK],
		ExampleStatus: map[bool]string{true: "ok", false: "fail"}[exampleOK],
		LastError:     lastError,
		Version:       version,
		ResponseMs:    elapsed,
	}
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
