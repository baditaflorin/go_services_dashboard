package services

import (
	"embed"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/baditaflorin/go_services_dashboard/models"
)

// Registry holds all services
type Registry struct {
	services map[string]*models.Service
	mu       sync.RWMutex
}

// NewRegistry initializes the registry with services from the embedded config
func NewRegistry(configFS embed.FS) (*Registry, error) {
	r := &Registry{
		services: make(map[string]*models.Service),
	}

	// Load from embedded file
	data, err := configFS.ReadFile("config/services.json")
	if err != nil {
		return nil, err
	}

	var servicesList []models.Service
	if err := json.Unmarshal(data, &servicesList); err != nil {
		return nil, err
	}

	for _, s := range servicesList {
		// Create a copy to store as pointer
		svc := s
		r.services[s.ID] = &svc
	}

	return r, nil
}

// GetAll returns all services, optionally filtered or sorted (implementation specific)
func (r *Registry) GetAll() []*models.Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*models.Service, 0, len(r.services))
	for _, s := range r.services {
		list = append(list, s)
	}

	// Sort by name by default
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})

	return list
}

// GetByID returns a single service by ID
func (r *Registry) GetByID(id string) (*models.Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.services[id]
	return s, ok
}

// UpdateStatus updates the health status of a service
func (r *Registry) UpdateStatus(id string, status string, responseMs int64, version string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if s, ok := r.services[id]; ok {
		s.Status = status
		s.ResponseMs = responseMs
		if version != "" {
			s.Version = version
		}
		s.LastChecked = time.Now() 
	}
}
