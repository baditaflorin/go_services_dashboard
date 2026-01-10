package checker

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// GetInternalHosts returns possible container names
func GetInternalHosts(svc *models.Service) []string {
	names := []string{}
	// Prioritize localhost for host-networking setups
	names = append(names, "localhost")

	if svc.DockerName != "" {
		names = append(names, svc.DockerName)
	}
	if svc.ID != "" && svc.ID != svc.DockerName {
		names = append(names, svc.ID+"-app-1")
	}
	// Deduplicate
	unique := make([]string, 0, len(names))
	seen := make(map[string]bool)
	for _, n := range names {
		if !seen[n] && n != "" {
			unique = append(unique, n)
			seen[n] = true
		}
	}
	return unique
}

// GetInternalPorts returns possible ports
func GetInternalPorts(svc *models.Service) []int {
	ports := []int{}
	if svc.Port > 0 {
		ports = append(ports, svc.Port)
	}
	// Always check 8080 as backup if different
	if svc.Port != 8080 {
		ports = append(ports, 8080)
	}
	return ports
}

// GetPathFromURL extracts the path and query from a full URL
func GetPathFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fallback manual parsing if URL is partial or broken
		idx := strings.Index(rawURL, "://")
		if idx != -1 {
			rawURL = rawURL[idx+3:]
		}
		pathIdx := strings.Index(rawURL, "/")
		if pathIdx != -1 {
			return rawURL[pathIdx:]
		}
		return "/"
	}
	return u.RequestURI()
}

// TryInternalRequest attempts to reach the service via internal Docker DNS
// Returns response on first success (200-399 range) or last error
func TryInternalRequest(client *http.Client, svc *models.Service, path string) (*http.Response, string, error) {
	hosts := GetInternalHosts(svc)
	ports := GetInternalPorts(svc)
	var lastErr error
	var triedURLs []string

	for _, host := range hosts {
		for _, port := range ports {
			targetURL := fmt.Sprintf("http://%s:%d%s", host, port, path)
			triedURLs = append(triedURLs, targetURL)

			resp, err := client.Get(targetURL)
			if err == nil {
				if resp.StatusCode >= 200 && resp.StatusCode < 500 {
					return resp, targetURL, nil
				}
				resp.Body.Close()
				lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			} else {
				lastErr = err
			}
		}
	}
	return nil, "", lastErr
}
