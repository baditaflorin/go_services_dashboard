package compliance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

type ValidationResult struct {
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

type ComplianceReport struct {
	ServiceID       string           `json:"service_id"`
	StandardPort    ValidationResult `json:"standard_port"`
	HealthFormat    ValidationResult `json:"health_format"`
	VersionEndpoint ValidationResult `json:"version_endpoint"`
	TotalScore      int              `json:"total_score"` // 0-100
	LastChecked     time.Time        `json:"last_checked"`
}

// ExpectedHealth structure for standardization
type ExpectedHealth struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func Scan(client *http.Client, svc *models.Service) ComplianceReport {
	report := ComplianceReport{
		ServiceID:   svc.ID,
		LastChecked: time.Now(),
	}
	score := 0
	maxScore := 3

	// 1. Standard Port Check
	// Penalize 8080 (Common conflict)
	if svc.Port == 8080 {
		report.StandardPort = ValidationResult{Passed: false, Reason: "Uses default 8080 (High Conflict Risk)"}
	} else {
		report.StandardPort = ValidationResult{Passed: true}
		score++
	}

	// 2. Health Endpoint Format Check
	if svc.HealthURL == "" {
		report.HealthFormat = ValidationResult{Passed: false, Reason: "No Health URL configured"}
	} else {
		resp, err := client.Get(svc.HealthURL)
		if err != nil {
			report.HealthFormat = ValidationResult{Passed: false, Reason: fmt.Sprintf("Unreachable: %v", err)}
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				report.HealthFormat = ValidationResult{Passed: false, Reason: fmt.Sprintf("HTTP %d", resp.StatusCode)}
			} else {
				// Parse JSON
				var h ExpectedHealth
				if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
					report.HealthFormat = ValidationResult{Passed: false, Reason: "Invalid JSON or Non-Standard Format"}
				} else if h.Status == "" || h.Service == "" {
					report.HealthFormat = ValidationResult{Passed: false, Reason: "Missing standard keys (status, service)"}
				} else {
					report.HealthFormat = ValidationResult{Passed: true}
					score++
				}
			}
		}
	}

	// 3. Version Endpoint Check (Assumes /version logic similar to Health)
	// We infer version existence if svc.Version is populated, but let's verify standard endpoint if needed.
	// We'll rely on svc.Version for now which Checker populates.
	if svc.Version != "" {
		report.VersionEndpoint = ValidationResult{Passed: true, Reason: "Version detected"}
		score++
	} else {
		report.VersionEndpoint = ValidationResult{Passed: false, Reason: "No Version detected"}
	}

	report.TotalScore = int((float64(score) / float64(maxScore)) * 100)
	return report
}
