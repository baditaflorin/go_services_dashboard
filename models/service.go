package models

import "time"

// Service represents a monitored microservice
type Service struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Category    string    `json:"category"` // domains, security, recon, infrastructure, web_analysis
	Port        int       `json:"port"`
	DockerName  string    `json:"docker_name"`
	RepoURL     string    `json:"repo_url"`
	ExampleURL  string    `json:"example_url"`
	HealthURL   string    `json:"health_url"`
	Status      string    `json:"status"` // healthy, unhealthy, unknown
	Version     string    `json:"version"`
	LastChecked time.Time `json:"last_checked"`
	Uptime      float64   `json:"uptime_percent"`
	ResponseMs  int64     `json:"response_ms"`
	Tags        []string  `json:"tags"`
	TestStatus  int       `json:"test_status"`  // HTTP status code from test link
	TestChecked time.Time `json:"test_checked"` // When test was last validated
}
