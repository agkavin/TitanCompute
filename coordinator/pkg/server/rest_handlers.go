package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// REST API request/response structures

// InferenceRequestREST represents a REST API inference request
type InferenceRequestREST struct {
	ClientID    string            `json:"client_id"`
	Model       string            `json:"model"`
	Prompt      string            `json:"prompt"`
	MaxTokens   int32             `json:"max_tokens,omitempty"`
	Temperature float32           `json:"temperature,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// InferenceResponseREST represents a REST API inference response
type InferenceResponseREST struct {
	AgentEndpoint  string  `json:"agent_endpoint"`
	SessionToken   string  `json:"session_token"`
	ExpiresAt      int64   `json:"expires_at"`
	JobID          string  `json:"job_id"`
	EstimatedRTTMs float64 `json:"estimated_rtt_ms"`
	AgentID        string  `json:"agent_id"`
}

// SystemStatusREST represents system status for REST API
type SystemStatusREST struct {
	TotalAgents   int               `json:"total_agents"`
	HealthyAgents int               `json:"healthy_agents"`
	Agents        []AgentStatusREST `json:"agents,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
}

// AgentStatusREST represents agent status for REST API
type AgentStatusREST struct {
	ID              string    `json:"id"`
	Endpoint        string    `json:"endpoint"`
	Status          string    `json:"status"`
	TotalVRAMMB     int64     `json:"total_vram_mb"`
	FreeVRAMMB      int64     `json:"free_vram_mb"`
	TotalRAMMB      int64     `json:"total_ram_mb"`
	FreeRAMMB       int64     `json:"free_ram_mb"`
	RunningJobs     int32     `json:"running_jobs"`
	QueuedJobs      int32     `json:"queued_jobs"`
	MaxJobs         int32     `json:"max_jobs"`
	RTTMs           float64   `json:"rtt_ms"`
	LastHeartbeat   time.Time `json:"last_heartbeat"`
	SupportedModels []string  `json:"supported_models"`
}

// ErrorResponseREST represents an error response
type ErrorResponseREST struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// RESTHandler handles HTTP REST API requests
type RESTHandler struct {
	coordinatorServer *CoordinatorServer
	logger            *logrus.Logger
}

// NewRESTHandler creates a new REST handler
func NewRESTHandler(coordinatorServer *CoordinatorServer, logger *logrus.Logger) *RESTHandler {
	return &RESTHandler{
		coordinatorServer: coordinatorServer,
		logger:            logger,
	}
}

// SetupRoutes configures the REST API routes
func (h *RESTHandler) SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// API v1 routes
	v1 := router.PathPrefix("/api/v1").Subrouter()

	// Inference endpoints
	v1.HandleFunc("/inference/request", h.handleInferenceRequest).Methods("POST")

	// System endpoints
	v1.HandleFunc("/health", h.handleHealth).Methods("GET")
	v1.HandleFunc("/status", h.handleSystemStatus).Methods("GET")

	// CORS middleware
	router.Use(h.corsMiddleware)

	// Logging middleware
	router.Use(h.loggingMiddleware)

	return router
}

// handleInferenceRequest handles POST /api/v1/inference/request
func (h *RESTHandler) handleInferenceRequest(w http.ResponseWriter, r *http.Request) {
	var req InferenceRequestREST

	// Parse JSON request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendErrorResponse(w, http.StatusBadRequest, "invalid_json", "Invalid JSON in request body")
		return
	}

	// Validate required fields
	if req.ClientID == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "missing_client_id", "client_id is required")
		return
	}
	if req.Model == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "missing_model", "model is required")
		return
	}
	if req.Prompt == "" {
		h.sendErrorResponse(w, http.StatusBadRequest, "missing_prompt", "prompt is required")
		return
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 100
	}
	if req.Temperature == 0 {
		req.Temperature = 0.7
	}

	h.logger.WithFields(logrus.Fields{
		"client_id":  req.ClientID,
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		"method":     "REST",
	}).Info("REST API inference request received")

	// Select optimal agent using existing scheduler
	agent, err := h.coordinatorServer.scheduler.SelectAgent(req.Model)
	if err != nil {
		h.logger.WithError(err).Error("Agent selection failed")
		h.sendErrorResponse(w, http.StatusServiceUnavailable, "no_agents", "No agents available for the requested model")
		return
	}

	// Generate JWT session token using existing method
	token, err := h.coordinatorServer.generateSessionToken(agent.ID, req.ClientID, req.Model)
	if err != nil {
		h.logger.WithError(err).Error("Failed to generate session token")
		h.sendErrorResponse(w, http.StatusInternalServerError, "token_generation_failed", "Failed to generate session token")
		return
	}

	// Create job ID for tracking
	jobID := generateJobID()

	// Create response
	response := InferenceResponseREST{
		AgentEndpoint:  agent.Endpoint,
		SessionToken:   token.JWTToken,
		ExpiresAt:      token.ExpiresAt.Unix(),
		JobID:          jobID,
		EstimatedRTTMs: agent.RTTMs,
		AgentID:        agent.ID,
	}

	h.logger.WithFields(logrus.Fields{
		"agent_id":       agent.ID,
		"agent_endpoint": agent.Endpoint,
		"job_id":         jobID,
		"session_token":  token.ID,
		"method":         "REST",
	}).Info("REST API inference request routed")

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleHealth handles GET /api/v1/health
func (h *RESTHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "titancompute-coordinator",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleSystemStatus handles GET /api/v1/status
func (h *RESTHandler) handleSystemStatus(w http.ResponseWriter, r *http.Request) {
	// Get include_agents parameter
	includeAgents := r.URL.Query().Get("include_agents") == "true"

	// Get all agents from registry
	allAgents := h.coordinatorServer.registry.ListAllAgents()
	healthyAgents := h.coordinatorServer.registry.ListHealthyAgents()

	response := SystemStatusREST{
		TotalAgents:   len(allAgents),
		HealthyAgents: len(healthyAgents),
		Timestamp:     time.Now(),
	}

	// Include agent details if requested
	if includeAgents {
		response.Agents = make([]AgentStatusREST, len(allAgents))
		for i, agent := range allAgents {
			response.Agents[i] = AgentStatusREST{
				ID:              agent.ID,
				Endpoint:        agent.Endpoint,
				Status:          string(agent.Status),
				TotalVRAMMB:     agent.TotalVRAMMB,
				FreeVRAMMB:      agent.FreeVRAMMB,
				TotalRAMMB:      agent.TotalRAMMB,
				FreeRAMMB:       agent.FreeRAMMB,
				RunningJobs:     agent.RunningJobs,
				QueuedJobs:      agent.QueuedJobs,
				MaxJobs:         agent.MaxJobs,
				RTTMs:           agent.RTTMs,
				LastHeartbeat:   agent.LastHeartbeat,
				SupportedModels: agent.SupportedModels,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends a JSON error response
func (h *RESTHandler) sendErrorResponse(w http.ResponseWriter, statusCode int, errorCode, message string) {
	response := ErrorResponseREST{
		Error:   errorCode,
		Code:    statusCode,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// corsMiddleware handles CORS headers
func (h *RESTHandler) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func (h *RESTHandler) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		h.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"path":        r.URL.Path,
			"remote_addr": r.RemoteAddr,
			"user_agent":  r.UserAgent(),
			"duration_ms": time.Since(start).Milliseconds(),
		}).Info("HTTP request processed")
	})
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return "job_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
