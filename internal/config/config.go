package config

import (
	"encoding/json"
	"log"
	"os"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

func LoadServices(registry *models.Registry) {
	// Add self
	registry.AddService(&models.Service{
		ID:          "services-dashboard",
		Name:        "services-dashboard",
		Category:    "domains",
		Port:        8131, // Internal port really, externally 43565 often mapped
		Status:      "unknown",
		HealthURL:   "http://localhost:43565/health", // Self check
		Description: "The main dashboard",
		Tags:        []string{"dashboard", "infrastructure"},
	})

	// Load from config/services.json
	// Try multiple paths for robustness (container vs local)
	paths := []string{"config/services.json", "../config/services.json", "./services.json"}
	var content []byte
	var err error

	for _, p := range paths {
		content, err = os.ReadFile(p)
		if err == nil {
			log.Printf("Loaded config from %s", p)
			break
		}
	}

	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return
	}

	var config struct {
		Services []models.Service `json:"services"`
	}

	// Try unmarshalling object first
	if err := json.Unmarshal(content, &config); err != nil {
		// Fallback: array?
		var services []models.Service
		if err2 := json.Unmarshal(content, &services); err2 == nil {
			config.Services = services
		} else {
			log.Printf("Error parsing config file: %v", err)
			return
		}
	}

	for i := range config.Services {
		// Take address of index to avoid pointer sharing issues in loops if we weren't careful,
		// but here we just pass a pointer to the generic AddService
		s := config.Services[i]
		registry.AddService(&s)
	}
	log.Printf("Loaded %d services from config", len(config.Services))
}
