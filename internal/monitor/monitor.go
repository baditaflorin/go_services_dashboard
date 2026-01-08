package monitor

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/baditaflorin/go_services_dashboard/internal/checker"
	"github.com/baditaflorin/go_services_dashboard/internal/models"
)

// ServiceUpdate represents a real-time update for a service
type ServiceUpdate struct {
	ServiceID  string `json:"id"`
	Status     string `json:"status"`
	TestStatus string `json:"test_status,omitempty"`
	TestError  string `json:"test_error,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	ResponseMs int64  `json:"response_ms"`
}

// Monitor handles background health checking
type Monitor struct {
	registry  *models.Registry
	client    *http.Client
	interval  time.Duration
	clients   map[chan ServiceUpdate]bool
	clientsMu sync.RWMutex
}

// NewMonitor creates a new health monitor
func NewMonitor(r *models.Registry) *Monitor {
	return &Monitor{
		registry: r,
		client: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil // Follow redirects
			},
		},
		interval: 30 * time.Second,
		clients:  make(map[chan ServiceUpdate]bool),
	}
}

// Subscribe returns a channel for real-time updates
func (m *Monitor) Subscribe() chan ServiceUpdate {
	ch := make(chan ServiceUpdate, 50)
	m.clientsMu.Lock()
	m.clients[ch] = true
	m.clientsMu.Unlock()
	return ch
}

// Unsubscribe removes a client listener
func (m *Monitor) Unsubscribe(ch chan ServiceUpdate) {
	m.clientsMu.Lock()
	if _, ok := m.clients[ch]; ok {
		delete(m.clients, ch)
		close(ch)
	}
	m.clientsMu.Unlock()
}

func (m *Monitor) broadcast(update ServiceUpdate) {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()
	for ch := range m.clients {
		select {
		case ch <- update:
		default:
			// Skip slow clients to prevent blocking
		}
	}
}

// Start begins the monitoring loop
func (m *Monitor) Start() {
	// Initial check
	m.CheckAll()

	ticker := time.NewTicker(m.interval)
	for range ticker.C {
		m.CheckAll()
	}
}

func (m *Monitor) CheckAll() {
	services := m.registry.GetAll()
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20) // Limit concurrent checks

	for _, svc := range services {
		wg.Add(1)
		go func(s *models.Service) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			m.CheckService(s)
		}(svc)
	}

	wg.Wait()
	log.Printf("Health check completed for %d services", len(services))
}

func (m *Monitor) CheckService(svc *models.Service) {
	result := checker.CheckService(m.client, svc)

	m.registry.Mu.Lock()
	svc.LastChecked = time.Now()
	svc.ResponseMs = result.ResponseMs
	svc.Status = result.Status
	svc.HealthStatus = result.HealthStatus
	svc.ExampleStatus = result.ExampleStatus
	svc.LastError = result.LastError
	if result.Version != "" {
		svc.Version = result.Version
	}
	// Track health history (last 5 checks)
	svc.HealthHistory = append(svc.HealthHistory, result.Status)
	if len(svc.HealthHistory) > 5 {
		svc.HealthHistory = svc.HealthHistory[1:]
	}
	m.registry.Mu.Unlock()

	// Broadcast update
	m.broadcast(ServiceUpdate{
		ServiceID:  svc.ID,
		Status:     svc.Status,
		LastError:  svc.LastError,
		ResponseMs: svc.ResponseMs,
	})
}

// TestActiveLink tests if the service's ExampleURL is actually working
func (m *Monitor) TestActiveLink(id string) (string, string, error) {
	m.registry.Mu.RLock()
	svc, exists := m.registry.Services[id]
	m.registry.Mu.RUnlock()

	if !exists {
		return "", "", nil
	}

	result := checker.TestActiveLink(m.client, svc)

	m.registry.Mu.Lock()
	svc.TestStatus = result.Status
	svc.TestError = result.Error
	m.registry.Mu.Unlock()

	// Broadcast update including test result
	m.broadcast(ServiceUpdate{
		ServiceID:  svc.ID,
		Status:     svc.Status,
		TestStatus: result.Status,
		TestError:  result.Error,
		ResponseMs: svc.ResponseMs,
	})

	return result.Status, result.Error, nil
}
