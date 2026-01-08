package checker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

	// STEP 1: Test Internal /health endpoint
	resp, resolveURL, err := TryInternalRequest(client, svc, "/health")
	if err == nil && resp != nil && resp.StatusCode == 200 {
		healthOK = true
		version = parseVersion(resp)
		resp.Body.Close()

		// Update discovered connection details
		u, _ := url.Parse(resolveURL)
		if u != nil {
			svc.DockerName = u.Hostname()
			// Port parsing if needed, but TryInternalRequest iterates trusted ports
		}
	} else {
		if err != nil {
			healthError = fmt.Sprintf("Internal health: %v", err)
		} else if resp != nil {
			healthError = fmt.Sprintf("Internal health: HTTP %d", resp.StatusCode)
			resp.Body.Close()
		}

		// Fallback to public HealthURL
		if svc.HealthURL != "" {
			resp, err := client.Get(svc.HealthURL)
			if err == nil && resp.StatusCode == 200 {
				healthOK = true
				version = parseVersion(resp)
				resp.Body.Close()
			} else {
				if err != nil {
					healthError = fmt.Sprintf("%s | Public health: %v", healthError, err)
				} else if resp != nil {
					healthError = fmt.Sprintf("%s | Public health: HTTP %d", healthError, resp.StatusCode)
					resp.Body.Close()
				}
			}
		}
	}

	// STEP 2: Test ExampleURL (actual service functionality)
	var exampleStatusCode int
	if svc.ExampleURL != "" {
		// Try Internal First
		path := GetPathFromURL(svc.ExampleURL)
		resp, _, err := TryInternalRequest(client, svc, path)

		if err == nil && resp != nil {
			exampleStatusCode = resp.StatusCode
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				exampleOK = true
			} else {
				exampleError = fmt.Sprintf("Internal HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			resp.Body.Close()
		} else {
			// Internal failed, try Public
			// log.Printf("Internal ExampleURL failed for %s: %v. Trying public...", svc.ID, err)
			resp, err := client.Get(svc.ExampleURL)
			if err == nil {
				exampleStatusCode = resp.StatusCode
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					exampleOK = true
				} else {
					exampleError = fmt.Sprintf("Public HTTP %d: %s", resp.StatusCode, resp.Status)
				}
				resp.Body.Close()
			} else {
				exampleError = fmt.Sprintf("Connection: %v", err)
			}
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
