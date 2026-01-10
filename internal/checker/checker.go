package checker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
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
	if svc.ID == "go_a11y_quick" {
		// Use log.Printf to ensure stderr capture
		log.Printf("[DEBUG-CHECKER] CheckService called for %s (Port: %d)\n", svc.ID, svc.Port)
	}
	start := time.Now()

	healthOK := false
	exampleOK := false
	version := ""
	healthError := ""
	exampleError := ""

	// STEP 1: Test Internal /health endpoint
	resp, resolveURL, err := TryInternalRequest(client, svc, "/health")

	// DEBUG 8155
	if svc.Port == 8155 {
		if err != nil {
			log.Printf("[DEBUG-8155] TryInternalRequest failed: %v\n", err)
		} else {
			log.Printf("[DEBUG-8155] TryInternalRequest success: %s (Status: %d)\n", resolveURL, resp.StatusCode)
		}
	}

	if err == nil && resp != nil && resp.StatusCode == 200 {
		var healthResp struct {
			Status  string `json:"status"`
			Version string `json:"version"`
		}
		if decodeErr := json.NewDecoder(resp.Body).Decode(&healthResp); decodeErr == nil {
			if healthResp.Status == "healthy" || healthResp.Status == "ok" {
				healthOK = true
				version = healthResp.Version
			} else {
				healthError = fmt.Sprintf("Internal health status: %s", healthResp.Status)
				if svc.Port == 8155 {
					log.Printf("[DEBUG-8155] Status rejected: %s\n", healthResp.Status)
				}
			}
		} else {
			// If strictly JSON is required, this should fail. But historically we allowed 200 OK.
			// Let's assume healthy if 200 OK but verify logs.
			healthOK = true
		}
		resp.Body.Close()

		// Update discovered connection details
		u, _ := url.Parse(resolveURL)
		if u != nil {
			svc.DockerName = u.Hostname()
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
				// Check Public Status too
				var healthResp struct {
					Status  string `json:"status"`
					Version string `json:"version"`
				}
				if decodeErr := json.NewDecoder(resp.Body).Decode(&healthResp); decodeErr == nil {
					if healthResp.Status == "healthy" || healthResp.Status == "ok" {
						healthOK = true
						version = healthResp.Version
					}
				} else {
					healthOK = true
				}
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
		// 1. Try Public URL First (End-to-End Check)
		resp, err := client.Get(svc.ExampleURL)

		publicOK := false
		if err == nil {
			exampleStatusCode = resp.StatusCode
			ct := resp.Header.Get("Content-Type")
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				if strings.Contains(ct, "text/html") {
					exampleError = fmt.Sprintf("Public: Unexpected HTML (HTTP %d)", resp.StatusCode)
				} else {
					publicOK = true
					exampleOK = true
				}
			} else {
				exampleError = fmt.Sprintf("Public HTTP %d: %s", resp.StatusCode, resp.Status)
			}
			resp.Body.Close()
		} else {
			exampleError = fmt.Sprintf("Public Connection: %v", err)
		}

		// 2. If Public failed, Try Internal (Diagnosis)
		if !publicOK {
			path := GetPathFromURL(svc.ExampleURL)
			resp, _, err := TryInternalRequest(client, svc, path)
			if err == nil && resp != nil {
				// We have internal connectivity
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					// Internal is fine, but Public failed -> Mark as Healthy (Internal)
					exampleError = fmt.Sprintf("%s | Internal OK (HTTP %d)", exampleError, resp.StatusCode)
					exampleOK = true
				} else {
					exampleError = fmt.Sprintf("%s | Internal also failed (HTTP %d)", exampleError, resp.StatusCode)
				}
				resp.Body.Close()
			} else {
				exampleError = fmt.Sprintf("%s | Internal unreachable", exampleError)
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
