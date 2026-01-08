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
	start := time.Now()
	var resp *http.Response
	var err error

	// PRIORITY 1: Test via internal Docker endpoint
	if svc.ExampleURL != "" {
		path := GetPathFromURL(svc.ExampleURL)

		resp, _, err = TryInternalRequest(client, svc, path)
		if err == nil {
			defer resp.Body.Close()
			elapsed := time.Since(start).Milliseconds()

			// Check if response is valid JSON
			bodyBytes, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				var jsonCheck map[string]interface{}
				if json.Unmarshal(bodyBytes, &jsonCheck) == nil {
					if _, hasResult := jsonCheck["result"]; hasResult {
						return TestServiceResult{Status: "passing", Error: fmt.Sprintf("OK in %dms", elapsed)}
					} else if _, hasTool := jsonCheck["tool"]; hasTool {
						return TestServiceResult{Status: "passing", Error: fmt.Sprintf("OK in %dms", elapsed)}
					}
				}
				// Valid HTTP but not expected JSON (relaxed check)
				return TestServiceResult{Status: "passing", Error: fmt.Sprintf("HTTP %d in %dms", resp.StatusCode, elapsed)}
			}
			return TestServiceResult{Status: "failed", Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)}
		}
		return TestServiceResult{Status: "failed", Error: fmt.Sprintf("Connection failed: %v", err)}
	}

	// FALLBACK: If no ExampleURL, test internal /health endpoint
	resp, _, err = TryInternalRequest(client, svc, "/health")
	if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
		resp.Body.Close()
		return TestServiceResult{Status: "passing", Error: "Health OK"}
	}

	return TestServiceResult{
		Status: "failed",
		Error:  "No ExampleURL configured, internal health check passed locally but failed validation",
	}
}
