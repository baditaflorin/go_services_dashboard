package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Service matches the dashboard's service model
type Service struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Port        int      `json:"port"`
	DockerName  string   `json:"docker_name"`
	RepoURL     string   `json:"repo_url"`
	ExampleURL  string   `json:"example_url"`
	HealthURL   string   `json:"health_url"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags"`
}

// Config file structure
type Config struct {
	Services []Service `json:"services"`
}

var (
	// Categories to scan
	categories = []string{"domains", "security", "recon", "infrastructure", "web_analysis"}
	// Regex to extract port from docker-compose
	portRegex = regexp.MustCompile(`"(\d+):\d+"`)
	// Regex for image name to guess service name if needed
	imageRegex = regexp.MustCompile(`image: ghcr.io/baditaflorin/(go_[a-zA-Z0-9_]+):latest`)
)

func main() {
	rootDir := "../" // Assuming running from domains/go_services_dashboard/scripts or similar, adjust as needed.
	// Actually, let's assume we run this from the root of the repo for simplicity, or handle pathing.
	// I will hardcode the root to the workspace path for certainty in this environment
	rootDir = "/Users/live/Documents/GITHUB_PROJECTS/scrape_hub"

	var services []Service

	for _, category := range categories {
		categoryPath := filepath.Join(rootDir, category)
		entries, err := os.ReadDir(categoryPath)
		if err != nil {
			log.Printf("Warning: could not read category %s: %v", category, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// We expect services to start with go_
			if !strings.HasPrefix(entry.Name(), "go_") {
				continue
			}

			serviceName := entry.Name()
			servicePath := filepath.Join(categoryPath, serviceName)
			composePath := filepath.Join(servicePath, "docker-compose.yml")

			// 1. Try to read .env file first
			envPath := filepath.Join(servicePath, ".env")
			port := 0

			if envContent, err := os.ReadFile(envPath); err == nil {
				// Simple parsing of .env looking for PORT=xxxxx
				re := regexp.MustCompile(`(?m)^PORT=(\d+)`)
				matches := re.FindStringSubmatch(string(envContent))
				if len(matches) > 1 {
					port, _ = strconv.Atoi(matches[1])
				}
			}

			// 2. If not found in .env, try docker-compose
			if port == 0 {
				if _, err := os.Stat(composePath); err == nil {
					content, err := os.ReadFile(composePath)
					if err != nil {
						log.Printf("Error reading %s: %v", composePath, err)
						continue
					}

					contentStr := string(content)

					// Strategy A: Look for ${PORT:-8104}
					defRegex := regexp.MustCompile(`\$\{PORT:-(\d+)\}`)
					matches := defRegex.FindStringSubmatch(contentStr)
					if len(matches) > 1 {
						port, _ = strconv.Atoi(matches[1])
					}

					// Strategy B: Look for standard "8080:8080" or 8080:8080 (ignoring optional quotes)
					if port == 0 {
						stdRegex := regexp.MustCompile(`(?m)^\s*-\s*"?(\d+):\d+"?`)
						matches := stdRegex.FindStringSubmatch(contentStr)
						if len(matches) > 1 {
							port, _ = strconv.Atoi(matches[1])
						}
					}
				}
			}

			if port != 0 {
				// Format display name: go_phone_extractor -> Phone Extractor
				displayName := strings.Title(strings.ReplaceAll(strings.TrimPrefix(serviceName, "go_"), "_", " "))

				// Format public hostname part: go_phone_extractor -> phone-extractor
				publicName := strings.ReplaceAll(strings.TrimPrefix(serviceName, "go_"), "_", "-")

				// Read metadata if exists
				metaPath := filepath.Join(servicePath, "service_metadata.json")
				testPath := "/?url=https://example.com" // Default fallback

				if metaContent, err := os.ReadFile(metaPath); err == nil {
					var meta struct {
						TestEndpoint string `json:"test_endpoint"`
					}
					if err := json.Unmarshal(metaContent, &meta); err == nil && meta.TestEndpoint != "" {
						testPath = meta.TestEndpoint
					}
				}

				service := Service{
					ID:          serviceName,
					Name:        serviceName,
					DisplayName: displayName,
					Description: fmt.Sprintf("Microservice for %s", displayName), // Placeholder
					Category:    category,
					Port:        port,
					DockerName:  fmt.Sprintf("%s-app-1", serviceName), // Standard naming convention in this repo
					RepoURL:     fmt.Sprintf("https://github.com/baditaflorin/%s", serviceName),
					ExampleURL:  fmt.Sprintf("https://%s.0crawl.com%s", publicName, testPath),
					HealthURL:   fmt.Sprintf("https://%s.0crawl.com/health", publicName),
					Status:      "unknown",
					Tags:        []string{"go", category, "microservice"},
				}
				services = append(services, service)
				fmt.Printf("Found service: %s on port %d\n", serviceName, port)
			}
		}
	}

	// Write to config/services.json
	outputDir := "/Users/live/Documents/GITHUB_PROJECTS/scrape_hub/domains/go_services_dashboard/config"
	fmt.Printf("Creating output directory: %s\n", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create dir: %v", err)
	}

	outputPath := filepath.Join(outputDir, "services.json")
	fmt.Printf("Creating output file: %s\n", outputPath)
	file, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(services); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Successfully generated configuration for %d services at %s\n", len(services), outputPath)
}
