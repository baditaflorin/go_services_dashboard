package monitor

import (
	"fmt"
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
	log.Printf("Starting health check cycle for %d services...", len(services))

	// Worker Pool: Limit concurrent checks to prevent network exhaustion
	numWorkers := 10
	jobs := make(chan *models.Service, len(services))
	var wg sync.WaitGroup

	// Start workers
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for svc := range jobs {
				log.Printf("[DEBUG-WORKER] Processing %s", svc.ID)
				m.CheckService(svc)
			}
		}()
	}

	// Enqueue jobs
	for _, svc := range services {
		jobs <- svc
	}
	close(jobs)

	wg.Wait()
	log.Printf("Health check cycle completed.")
}

func (m *Monitor) CheckService(svc *models.Service) {
	m.registry.Mu.Lock()
	// Circuit Breaker Check
	if !svc.CircuitOpenUntil.IsZero() && time.Now().Before(svc.CircuitOpenUntil) {
		svc.Status = "unhealthy"
		svc.LastError = fmt.Sprintf("Circuit Open (cooling down until %s)", svc.CircuitOpenUntil.Format("15:04:05"))
		m.registry.Mu.Unlock()
		return
	}
	m.registry.Mu.Unlock()

	// Retry Logic: Try up to 3 times (0s, 1s, 2s wait)
	var result checker.CheckServiceResult
	for attempt := 0; attempt < 3; attempt++ {
		result = checker.CheckService(m.client, svc)
		if result.Status == "healthy" {
			break
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

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

	// Circuit Breaker State Update
	if result.Status != "healthy" {
		svc.ConsecutiveFailures++
		if svc.ConsecutiveFailures >= 5 {
			svc.CircuitOpenUntil = time.Now().Add(5 * time.Minute)
			svc.LastError = "Circuit Breaker Tripped (5 failing checks)"
		}
	} else {
		svc.ConsecutiveFailures = 0
		svc.CircuitOpenUntil = time.Time{}
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
