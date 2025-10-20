package registry

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// AgentStatus represents the status of an agent with circuit breaker support
type AgentStatus string

const (
	StatusHealthy  AgentStatus = "healthy"   // Fully operational
	StatusDegraded AgentStatus = "degraded"  // Experiencing issues but operational
	StatusHalfOpen AgentStatus = "half_open" // Circuit breaker half-open, limited traffic
	StatusOffline  AgentStatus = "offline"   // Circuit breaker open, no traffic
)

// AgentInfo holds information about a registered agent
type AgentInfo struct {
	ID              string            `json:"id"`
	Endpoint        string            `json:"endpoint"`
	TotalVRAMMB     int64             `json:"total_vram_mb"`
	TotalRAMMB      int64             `json:"total_ram_mb"`
	MaxJobs         int32             `json:"max_jobs"`
	SupportedModels []string          `json:"supported_models"`
	Capabilities    map[string]string `json:"capabilities"`

	// Runtime metrics
	FreeVRAMMB    int64     `json:"free_vram_mb"`
	FreeRAMMB     int64     `json:"free_ram_mb"`
	RunningJobs   int32     `json:"running_jobs"`
	QueuedJobs    int32     `json:"queued_jobs"`
	CPUPercent    float64   `json:"cpu_percent"`
	RTTMs         float64   `json:"rtt_ms"`
	LastHeartbeat time.Time `json:"last_heartbeat"`

	// Status and Circuit Breaker
	Status       AgentStatus `json:"status"`
	RegisteredAt time.Time   `json:"registered_at"`

	// Circuit Breaker State
	FailureCount    int32     `json:"failure_count"`
	LastFailureTime time.Time `json:"last_failure_time"`
	NextRetryTime   time.Time `json:"next_retry_time"`
	SuccessCount    int32     `json:"success_count"`
}

// HealthStats represents health metrics from an agent
type HealthStats struct {
	AgentID     string
	FreeVRAMMB  int64
	FreeRAMMB   int64
	RunningJobs int32
	QueuedJobs  int32
	CPUPercent  float64
	RTTMs       float64
	Timestamp   int64
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold int32         // Failures before opening circuit
	RecoveryTimeout  time.Duration // Time before attempting half-open
	SuccessThreshold int32         // Successes needed to close circuit
	HalfOpenTimeout  time.Duration // Max time in half-open state
}

// DefaultCircuitBreakerConfig returns sensible defaults
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 3,                // 3 failures triggers open
		RecoveryTimeout:  30 * time.Second, // 30s before half-open attempt
		SuccessThreshold: 2,                // 2 successes closes circuit
		HalfOpenTimeout:  10 * time.Second, // 10s max in half-open
	}
}

// Registry defines the interface for agent registration
type Registry interface {
	Register(agent *AgentInfo) error
	Deregister(agentID string) error
	UpdateHealth(agentID string, stats *HealthStats) error
	GetAgent(agentID string) (*AgentInfo, error)
	ListHealthyAgents() []*AgentInfo
	ListAllAgents() []*AgentInfo
	GetStats() (int, int) // total, healthy

	// Circuit Breaker methods
	RecordSuccess(agentID string) error
	RecordFailure(agentID string) error
	GetCircuitState(agentID string) (AgentStatus, error)
}

// InMemoryRegistry implements Registry using in-memory storage
type InMemoryRegistry struct {
	mu               sync.RWMutex
	agents           map[string]*AgentInfo
	heartbeatTimeout time.Duration
	cleanupInterval  time.Duration
	logger           *logrus.Logger
	stopCleanup      chan struct{}
	circuitConfig    CircuitBreakerConfig
}

// NewInMemoryRegistry creates a new in-memory registry
func NewInMemoryRegistry(heartbeatTimeout time.Duration, logger *logrus.Logger) *InMemoryRegistry {
	return &InMemoryRegistry{
		agents:           make(map[string]*AgentInfo),
		heartbeatTimeout: heartbeatTimeout,
		cleanupInterval:  60 * time.Second,
		logger:           logger,
		stopCleanup:      make(chan struct{}),
		circuitConfig:    DefaultCircuitBreakerConfig(),
	}
}

// Register adds a new agent to the registry
func (r *InMemoryRegistry) Register(agent *AgentInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent.RegisteredAt = time.Now()
	agent.LastHeartbeat = time.Now()
	agent.Status = StatusHealthy

	r.agents[agent.ID] = agent

	r.logger.WithFields(logrus.Fields{
		"agent_id": agent.ID,
		"endpoint": agent.Endpoint,
		"vram_mb":  agent.TotalVRAMMB,
		"max_jobs": agent.MaxJobs,
	}).Info("Agent registered")

	return nil
}

// Deregister removes an agent from the registry
func (r *InMemoryRegistry) Deregister(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.agents[agentID]; !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	delete(r.agents, agentID)

	r.logger.WithField("agent_id", agentID).Info("Agent deregistered")
	return nil
}

// UpdateHealth updates the health metrics for an agent
func (r *InMemoryRegistry) UpdateHealth(agentID string, stats *HealthStats) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not registered", agentID)
	}

	// Update metrics
	agent.FreeVRAMMB = stats.FreeVRAMMB
	agent.FreeRAMMB = stats.FreeRAMMB
	agent.RunningJobs = stats.RunningJobs
	agent.QueuedJobs = stats.QueuedJobs
	agent.CPUPercent = stats.CPUPercent
	agent.RTTMs = stats.RTTMs
	agent.LastHeartbeat = time.Now()

	// Update status based on metrics (only for healthy/degraded agents)
	if agent.Status == StatusHealthy || agent.Status == StatusDegraded {
		// Check for degradation conditions
		vramUtilization := float64(agent.TotalVRAMMB-agent.FreeVRAMMB) / float64(agent.TotalVRAMMB)
		if vramUtilization > 0.9 || agent.CPUPercent > 90.0 || agent.FreeVRAMMB < 512 {
			if agent.Status == StatusHealthy {
				agent.Status = StatusDegraded
				r.logger.WithFields(logrus.Fields{
					"agent_id":     agentID,
					"vram_util":    vramUtilization,
					"cpu_percent":  agent.CPUPercent,
					"free_vram_mb": agent.FreeVRAMMB,
				}).Warn("Agent degraded due to resource constraints")
			}
		} else if agent.Status == StatusDegraded && vramUtilization < 0.7 && agent.CPUPercent < 70.0 {
			agent.Status = StatusHealthy
			r.logger.WithField("agent_id", agentID).Info("Agent recovered to healthy status")
		}
	}

	return nil
}

// GetAgent retrieves a specific agent by ID
func (r *InMemoryRegistry) GetAgent(agentID string) (*AgentInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", agentID)
	}

	// Return a copy to avoid external mutations
	agentCopy := *agent
	return &agentCopy, nil
}

// ListHealthyAgents returns all healthy agents
func (r *InMemoryRegistry) ListHealthyAgents() []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthy := make([]*AgentInfo, 0)
	for _, agent := range r.agents {
		if agent.Status == StatusHealthy {
			agentCopy := *agent
			healthy = append(healthy, &agentCopy)
		}
	}

	return healthy
}

// ListAllAgents returns all agents regardless of status
func (r *InMemoryRegistry) ListAllAgents() []*AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]*AgentInfo, 0, len(r.agents))
	for _, agent := range r.agents {
		agentCopy := *agent
		all = append(all, &agentCopy)
	}

	return all
}

// GetStats returns total and healthy agent counts
func (r *InMemoryRegistry) GetStats() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.agents)
	healthy := 0

	for _, agent := range r.agents {
		if agent.Status == StatusHealthy {
			healthy++
		}
	}

	return total, healthy
}

// StartCleanup starts the background cleanup routine
func (r *InMemoryRegistry) StartCleanup() {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupStaleAgents()
		case <-r.stopCleanup:
			return
		}
	}
}

// cleanupStaleAgents removes agents that haven't sent heartbeats
func (r *InMemoryRegistry) cleanupStaleAgents() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	staleAgents := make([]string, 0)

	for agentID, agent := range r.agents {
		// Check heartbeat timeout
		if now.Sub(agent.LastHeartbeat) > r.heartbeatTimeout {
			agent.Status = StatusOffline
			staleAgents = append(staleAgents, agentID)
		}

		// Check circuit breaker state transitions
		r.updateCircuitBreakerState(agent, now)
	}

	// Remove offline agents
	for _, agentID := range staleAgents {
		delete(r.agents, agentID)
		r.logger.WithField("agent_id", agentID).Warn("Agent removed due to missed heartbeats")
	}

	if len(staleAgents) > 0 {
		r.logger.WithField("removed_count", len(staleAgents)).Info("Cleanup completed")
	}
}

// RecordSuccess records a successful operation for circuit breaker
func (r *InMemoryRegistry) RecordSuccess(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.SuccessCount++
	agent.FailureCount = 0 // Reset failure count on success

	// Check for circuit breaker state transitions
	if agent.Status == StatusHalfOpen && agent.SuccessCount >= r.circuitConfig.SuccessThreshold {
		agent.Status = StatusHealthy
		agent.SuccessCount = 0
		r.logger.WithField("agent_id", agentID).Info("Circuit breaker closed - agent recovered")
	}

	return nil
}

// RecordFailure records a failed operation for circuit breaker
func (r *InMemoryRegistry) RecordFailure(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return fmt.Errorf("agent %s not found", agentID)
	}

	agent.FailureCount++
	agent.LastFailureTime = time.Now()
	agent.SuccessCount = 0 // Reset success count on failure

	// Check if we should open the circuit
	if agent.FailureCount >= r.circuitConfig.FailureThreshold {
		agent.Status = StatusOffline
		agent.NextRetryTime = time.Now().Add(r.circuitConfig.RecoveryTimeout)
		r.logger.WithFields(logrus.Fields{
			"agent_id":      agentID,
			"failure_count": agent.FailureCount,
			"next_retry":    agent.NextRetryTime,
		}).Warn("Circuit breaker opened - agent marked offline")
	}

	return nil
}

// GetCircuitState returns the current circuit breaker state
func (r *InMemoryRegistry) GetCircuitState(agentID string) (AgentStatus, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, exists := r.agents[agentID]
	if !exists {
		return StatusOffline, fmt.Errorf("agent %s not found", agentID)
	}

	return agent.Status, nil
}

// updateCircuitBreakerState updates the circuit breaker state based on time
func (r *InMemoryRegistry) updateCircuitBreakerState(agent *AgentInfo, now time.Time) {
	switch agent.Status {
	case StatusOffline:
		// Check if it's time to try half-open
		if now.After(agent.NextRetryTime) {
			agent.Status = StatusHalfOpen
			agent.NextRetryTime = now.Add(r.circuitConfig.HalfOpenTimeout)
			r.logger.WithField("agent_id", agent.ID).Info("Circuit breaker half-open - limited retry enabled")
		}
	case StatusHalfOpen:
		// Check if half-open timeout expired
		if now.After(agent.NextRetryTime) {
			agent.Status = StatusOffline
			agent.NextRetryTime = now.Add(r.circuitConfig.RecoveryTimeout)
			r.logger.WithField("agent_id", agent.ID).Warn("Half-open timeout expired - circuit breaker reopened")
		}
	case StatusDegraded:
		// Check if degraded agent should recover
		if agent.FreeVRAMMB > 2048 && agent.CPUPercent < 80.0 {
			agent.Status = StatusHealthy
			agent.FailureCount = 0
			r.logger.WithField("agent_id", agent.ID).Info("Agent recovered from degraded state")
		}
	}
}
