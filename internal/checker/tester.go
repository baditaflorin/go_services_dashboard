package checker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// TestServiceResult holds the result of an active link test
type TestServiceResult struct {
	Status string
	Error  string
}

// TestActiveLink tests if the service's ExampleURL is actually working
func TestActiveLink(client *http.Client, svc *models.Service) TestServiceResult {
	status := "failed"
	errMsg := ""
	start := time.Now()

	// Build internal test URLs
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
	if svc.ExampleURL != "" {
		// Extract path from ExampleURL
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

					// Check if response is valid JSON
					bodyBytes, _ := io.ReadAll(resp.Body)
					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						var jsonCheck map[string]interface{}
						if json.Unmarshal(bodyBytes, &jsonCheck) == nil {
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

		// STRICT CHECK: IF EXAMPLE URL CONFIGURED, DO NOT FALLBACK
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
	return TestServiceResult{
		Status: status,
		Error:  errMsg,
	}
}
