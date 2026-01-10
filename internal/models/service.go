package models

import (
	"sync"
	"time"
)

// Service represents a monitored microservice
type Service struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	DisplayName         string    `json:"display_name"`
	Description         string    `json:"description"`
	Category            string    `json:"category"`
	Port                int       `json:"port"`
	DockerName          string    `json:"docker_name"`
	RepoURL             string    `json:"repo_url"`
	ExampleURL          string    `json:"example_url"`
	HealthURL           string    `json:"health_url"`
	Status              string    `json:"status"`         // healthy, degraded, unhealthy
	HealthStatus        string    `json:"health_status"`  // /health endpoint status
	ExampleStatus       string    `json:"example_status"` // ExampleURL status
	LastError           string    `json:"last_error,omitempty"`
	TestStatus          string    `json:"test_status"`
	TestError           string    `json:"test_error,omitempty"`
	Version             string    `json:"version"`
	LatestVersion       string    `json:"latest_version,omitempty"` // Latest available Docker image version
	UpdateAvailable     bool      `json:"update_available"`         // True if Version != LatestVersion
	LastChecked         time.Time `json:"last_checked"`
	ResponseMs          int64     `json:"response_ms"`
	Tags                []string  `json:"tags"`
	HealthHistory       []string  `json:"health_history,omitempty"`     // Last 5 checks
	ConsecutiveFailures int       `json:"-"`                            // Internal counter for circuit breaker
	CircuitOpenUntil    time.Time `json:"circuit_open_until,omitempty"` // When to try again if breaker is open
}

// Registry holds all services
type Registry struct {
	Services map[string]*Service
	Mu       sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		Services: make(map[string]*Service),
	}
}

func (r *Registry) AddService(s *Service) {
	r.Mu.Lock()
	defer r.Mu.Unlock()
	r.Services[s.ID] = s
}

func (r *Registry) GetAll() []*Service {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	list := make([]*Service, 0, len(r.Services))
	for _, s := range r.Services {
		list = append(list, s)
	}
	return list
}

func (r *Registry) Get(id string) (*Service, bool) {
	r.Mu.RLock()
	defer r.Mu.RUnlock()
	s, exists := r.Services[id]
	return s, exists
}
