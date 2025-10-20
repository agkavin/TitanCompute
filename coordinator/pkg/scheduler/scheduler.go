package scheduler

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/titancompute/coordinator/pkg/registry"
)

// Scheduler defines the interface for agent selection
type Scheduler interface {
	SelectAgent(model string) (*registry.AgentInfo, error)
}

// MCDAWeights defines the weights for Multi-Criteria Decision Analysis
type MCDAWeights struct {
	VRAMWeight        float64 // Memory availability weight (40%)
	JobLoadWeight     float64 // Current job load weight (30%)
	LatencyWeight     float64 // Network latency weight (20%)
	PerformanceWeight float64 // Historical performance weight (10%)
}

// DefaultMCDAWeights returns the default scoring weights
func DefaultMCDAWeights() MCDAWeights {
	return MCDAWeights{
		VRAMWeight:        0.40,
		JobLoadWeight:     0.30,
		LatencyWeight:     0.20,
		PerformanceWeight: 0.10,
	}
}

// AgentScore represents a scored agent for MCDA selection
type AgentScore struct {
	Agent     *registry.AgentInfo
	Score     float64
	VRAMScore float64
	LoadScore float64
	RTTScore  float64
	PerfScore float64
}

// MCDAScheduler implements Memory-Aware Multi-Criteria Decision Analysis scheduling
type MCDAScheduler struct {
	registry    registry.Registry
	logger      *logrus.Logger
	weights     MCDAWeights
	mu          sync.RWMutex
	perfHistory map[string]float64 // Agent ID -> avg tokens/sec
}

// NewMCDAScheduler creates a new MCDA scheduler for M2
func NewMCDAScheduler(registry registry.Registry, logger *logrus.Logger) *MCDAScheduler {
	return &MCDAScheduler{
		registry:    registry,
		logger:      logger,
		weights:     DefaultMCDAWeights(),
		perfHistory: make(map[string]float64),
	}
}

// RoundRobinScheduler implements simple round-robin scheduling for M1 compatibility
type RoundRobinScheduler struct {
	registry  registry.Registry
	logger    *logrus.Logger
	mu        sync.Mutex
	lastIndex int
}

// NewRoundRobinScheduler creates a new round-robin scheduler
func NewRoundRobinScheduler(registry registry.Registry, logger *logrus.Logger) *RoundRobinScheduler {
	return &RoundRobinScheduler{
		registry:  registry,
		logger:    logger,
		lastIndex: -1,
	}
}

// SelectAgent selects the optimal agent using MCDA scoring
func (s *MCDAScheduler) SelectAgent(model string) (*registry.AgentInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := s.registry.ListHealthyAgents()
	if len(agents) == 0 {
		return nil, fmt.Errorf("no healthy agents available")
	}

	// Filter and score compatible agents
	candidates := make([]*AgentScore, 0)
	for _, agent := range agents {
		if s.supportsModel(agent, model) {
			// Apply circuit breaker logic - only include healthy and half-open agents
			if agent.Status == registry.StatusHealthy || agent.Status == registry.StatusHalfOpen {
				score := s.calculateMCDAScore(agent)

				// Apply circuit breaker weight reduction for degraded agents
				if agent.Status == registry.StatusDegraded {
					score.Score *= 0.5
				}

				candidates = append(candidates, score)
			}
		}
	}

	if len(candidates) == 0 {
		s.logger.WithField("model", model).Warn("No compatible agents available, trying any healthy agent")
		for _, agent := range agents {
			if agent.Status == registry.StatusHealthy {
				score := s.calculateMCDAScore(agent)
				candidates = append(candidates, score)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable agents available")
	}

	// Sort by score (descending)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	selected := candidates[0]

	s.logger.WithFields(logrus.Fields{
		"agent_id":     selected.Agent.ID,
		"endpoint":     selected.Agent.Endpoint,
		"model":        model,
		"total_score":  selected.Score,
		"vram_score":   selected.VRAMScore,
		"load_score":   selected.LoadScore,
		"rtt_score":    selected.RTTScore,
		"perf_score":   selected.PerfScore,
		"free_vram_mb": selected.Agent.FreeVRAMMB,
		"running_jobs": selected.Agent.RunningJobs,
		"status":       selected.Agent.Status,
	}).Info("MCDA agent selected")

	return selected.Agent, nil
}

// SelectAgent selects the next agent using round-robin
func (s *RoundRobinScheduler) SelectAgent(model string) (*registry.AgentInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agents := s.registry.ListHealthyAgents()
	if len(agents) == 0 {
		return nil, fmt.Errorf("no healthy agents available")
	}

	// Filter agents that support the requested model
	compatible := make([]*registry.AgentInfo, 0)
	for _, agent := range agents {
		if s.supportsModel(agent, model) {
			compatible = append(compatible, agent)
		}
	}

	if len(compatible) == 0 {
		s.logger.WithField("model", model).Warn("No agents support requested model, using any available")
		compatible = agents
	}

	// Round-robin selection
	s.lastIndex = (s.lastIndex + 1) % len(compatible)
	selected := compatible[s.lastIndex]

	s.logger.WithFields(logrus.Fields{
		"agent_id":     selected.ID,
		"endpoint":     selected.Endpoint,
		"model":        model,
		"free_vram_mb": selected.FreeVRAMMB,
		"running_jobs": selected.RunningJobs,
	}).Info("Agent selected")

	return selected, nil
}

// calculateMCDAScore computes the multi-criteria score for an agent
func (s *MCDAScheduler) calculateMCDAScore(agent *registry.AgentInfo) *AgentScore {
	score := &AgentScore{Agent: agent}

	// VRAM Score (40% weight) - normalize to 0-1 based on available memory
	vramUtilization := float64(agent.TotalVRAMMB-agent.FreeVRAMMB) / math.Max(float64(agent.TotalVRAMMB), 1)
	score.VRAMScore = (1.0 - vramUtilization) // Higher free VRAM = better score

	// Job Load Score (30% weight) - normalize based on max jobs
	loadUtilization := float64(agent.RunningJobs) / math.Max(float64(agent.MaxJobs), 1)
	score.LoadScore = (1.0 - loadUtilization) // Lower load = better score

	// RTT Score (20% weight) - normalize RTT (lower is better)
	// Assume typical RTT range 1-500ms, normalize to 0-1
	normalizedRTT := math.Min(agent.RTTMs/500.0, 1.0)
	score.RTTScore = (1.0 - normalizedRTT)

	// Performance Score (10% weight) - based on historical throughput
	s.mu.RLock()
	avgPerf, exists := s.perfHistory[agent.ID]
	s.mu.RUnlock()

	if exists {
		// Normalize performance (assume 1-100 tokens/sec range)
		score.PerfScore = math.Min(avgPerf/100.0, 1.0)
	} else {
		score.PerfScore = 0.5 // Default neutral score for new agents
	}

	// Calculate weighted total score
	score.Score = (score.VRAMScore * s.weights.VRAMWeight) +
		(score.LoadScore * s.weights.JobLoadWeight) +
		(score.RTTScore * s.weights.LatencyWeight) +
		(score.PerfScore * s.weights.PerformanceWeight)

	return score
}

// UpdatePerformanceHistory updates the performance history for an agent
func (s *MCDAScheduler) UpdatePerformanceHistory(agentID string, tokensPerSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simple moving average (can be enhanced with more sophisticated metrics)
	if current, exists := s.perfHistory[agentID]; exists {
		s.perfHistory[agentID] = (current + tokensPerSec) / 2.0
	} else {
		s.perfHistory[agentID] = tokensPerSec
	}
}

// supportsModel checks if an agent supports a specific model (MCDA version)
func (s *MCDAScheduler) supportsModel(agent *registry.AgentInfo, model string) bool {
	// Enhanced model compatibility checking for M2

	// Circuit breaker check - exclude open circuit agents
	if agent.Status == registry.StatusOffline {
		return false
	}

	// Basic resource checks: agent should have sufficient free VRAM
	estimatedModelSize := s.estimateModelVRAMRequirement(model)
	if agent.FreeVRAMMB < estimatedModelSize {
		return false
	}

	// Check job capacity
	if agent.RunningJobs >= agent.MaxJobs {
		return false
	}

	// If agent has specific supported models list, check it
	if len(agent.SupportedModels) > 0 {
		for _, supportedModel := range agent.SupportedModels {
			if supportedModel == model {
				return true
			}
		}
		return false
	}

	// If no specific models listed, assume it can handle any model
	return true
}

// estimateModelVRAMRequirement estimates VRAM needed for a model
func (s *MCDAScheduler) estimateModelVRAMRequirement(model string) int64 {
	// Simple heuristic based on model name - can be enhanced with model registry
	// Default to 4GB for most models, adjust based on known patterns

	if model == "" {
		return 4096
	}

	// Look for size indicators in model name
	modelLower := model
	if len(modelLower) > 10 { // Reasonable length check
		if model[len(model)-3:] == "1B" || model[len(model)-2:] == "1B" {
			return 2048 // 2GB for 1B parameter models
		}
		if model[len(model)-3:] == "7B" || model[len(model)-2:] == "7B" {
			return 6144 // 6GB for 7B parameter models
		}
		if model[len(model)-4:] == "13B" || model[len(model)-3:] == "13B" {
			return 10240 // 10GB for 13B parameter models
		}
	}

	return 4096 // Default 4GB requirement
}

// supportsModel checks if an agent supports a specific model (RoundRobin version)
func (s *RoundRobinScheduler) supportsModel(agent *registry.AgentInfo, model string) bool {
	// For M1, we'll be lenient and assume all agents can load any model
	// This will be enhanced in M2 with proper model compatibility checking

	// Basic checks: agent should have some free VRAM and not be at max capacity
	if agent.FreeVRAMMB < 1024 { // At least 1GB free
		return false
	}

	if agent.RunningJobs >= agent.MaxJobs {
		return false
	}

	// If agent has specific supported models list, check it
	if len(agent.SupportedModels) > 0 {
		for _, supportedModel := range agent.SupportedModels {
			if supportedModel == model {
				return true
			}
		}
		return false
	}

	// If no specific models listed, assume it can handle any model
	return true
}
