package checker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// VersionChecker checks for available updates in Docker registry
type VersionChecker struct {
	client *http.Client
}

// NewVersionChecker creates a new version checker
func NewVersionChecker() *VersionChecker {
	return &VersionChecker{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GHCRTagsResponse represents the GHCR API response for tags
type GHCRTagsResponse struct {
	Tags []string `json:"tags"`
}

// CheckLatestVersion queries GHCR for the latest version of a service
func (vc *VersionChecker) CheckLatestVersion(svc *models.Service) {
	// Construct the GHCR API URL
	// Format: ghcr.io/baditaflorin/{service_name}
	imageName := svc.Name
	if imageName == "" {
		imageName = svc.ID
	}

	// Try multiple image naming patterns
	patterns := []string{
		fmt.Sprintf("ghcr.io/baditaflorin/%s", imageName),
		fmt.Sprintf("ghcr.io/baditaflorin/scrape_hub/%s", imageName),
	}

	for _, pattern := range patterns {
		tags, err := vc.fetchTags(pattern)
		if err == nil && len(tags) > 0 {
			latestVersion := vc.extractLatestVersion(tags)
			if latestVersion != "" {
				svc.LatestVersion = latestVersion
				svc.UpdateAvailable = svc.Version != "" && svc.Version != latestVersion && svc.Version != "1.0.0"
				return
			}
		}
	}

	// If no tags found, mark as unknown
	svc.LatestVersion = ""
	svc.UpdateAvailable = false
}

// fetchTags queries the GHCR API for available tags
func (vc *VersionChecker) fetchTags(imagePath string) ([]string, error) {
	// GHCR uses the OCI distribution API
	// GET /v2/{name}/tags/list
	parts := strings.SplitN(imagePath, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid image path: %s", imagePath)
	}

	registry := parts[0]
	name := parts[1]

	url := fmt.Sprintf("https://%s/v2/%s/tags/list", registry, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// GHCR requires authentication for private repos, but public repos should work
	// For now, we'll attempt without auth
	req.Header.Set("Accept", "application/json")

	resp, err := vc.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GHCR returned status %d", resp.StatusCode)
	}

	var tagsResp GHCRTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, err
	}

	return tagsResp.Tags, nil
}

// extractLatestVersion finds the highest semantic version from tags
func (vc *VersionChecker) extractLatestVersion(tags []string) string {
	// Filter for semantic version tags (e.g., "1.0.0", "1.2.3", "v1.0.0")
	semverRegex := regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)

	var versions []string
	for _, tag := range tags {
		if tag == "latest" {
			continue // Skip "latest" tag
		}
		if semverRegex.MatchString(tag) {
			// Normalize - remove 'v' prefix if present
			v := strings.TrimPrefix(tag, "v")
			versions = append(versions, v)
		}
	}

	if len(versions) == 0 {
		return ""
	}

	// Sort versions (simple string sort works for semver with same digit counts)
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})

	return versions[0]
}

// compareVersions compares two semver strings
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < 3; i++ {
		var p1, p2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &p1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &p2)
		}
		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}
	return 0
}
