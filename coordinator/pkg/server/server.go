package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/titancompute/coordinator/pkg/config"
	pb "github.com/titancompute/coordinator/pkg/proto/github.com/titancompute/proto/gen/go"
	"github.com/titancompute/coordinator/pkg/registry"
	"github.com/titancompute/coordinator/pkg/scheduler"
)

// JWTClaims represents the claims in our JWT tokens
type JWTClaims struct {
	AgentID  string `json:"agent_id"`
	ClientID string `json:"client_id"`
	Model    string `json:"model"`
	jwt.RegisteredClaims
}

// JWTManager handles JWT token operations
type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	logger     *logrus.Logger
	mu         sync.RWMutex
}

// CoordinatorServer implements the coordinator gRPC service
type CoordinatorServer struct {
	pb.UnimplementedCoordinatorServiceServer
	registry   registry.Registry
	scheduler  scheduler.Scheduler
	config     *config.Config
	logger     *logrus.Logger
	tokens     map[string]*SessionToken
	jwtManager *JWTManager
}

// SessionToken represents a session token for direct agent access
type SessionToken struct {
	ID        string
	AgentID   string
	ClientID  string
	Model     string
	IssuedAt  time.Time
	ExpiresAt time.Time
	JWTToken  string // The actual JWT token string
}

// NewJWTManager creates a new JWT manager with RSA key pair
func NewJWTManager(logger *logrus.Logger) (*JWTManager, error) {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	return &JWTManager{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		logger:     logger,
	}, nil
}

// GenerateToken creates a new JWT token
func (j *JWTManager) GenerateToken(agentID, clientID, model string, ttl time.Duration) (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	now := time.Now()
	claims := &JWTClaims{
		AgentID:  agentID,
		ClientID: clientID,
		Model:    model,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "titancompute-coordinator",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(j.privateKey)
}

// ValidateToken validates and parses a JWT token
func (j *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// GetPublicKeyPEM returns the public key in PEM format for agent validation
func (j *JWTManager) GetPublicKeyPEM() (string, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(j.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}

// NewCoordinatorServer creates a new coordinator server
func NewCoordinatorServer(
	registry registry.Registry,
	scheduler scheduler.Scheduler,
	config *config.Config,
	logger *logrus.Logger,
) (*CoordinatorServer, error) {
	jwtManager, err := NewJWTManager(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return &CoordinatorServer{
		registry:   registry,
		scheduler:  scheduler,
		config:     config,
		logger:     logger,
		tokens:     make(map[string]*SessionToken),
		jwtManager: jwtManager,
	}, nil
}

// RegisterCoordinatorService registers the coordinator service with gRPC server
func RegisterCoordinatorService(server *grpc.Server, coordinatorServer *CoordinatorServer) {
	pb.RegisterCoordinatorServiceServer(server, coordinatorServer)
}

// RequestInference handles inference routing requests
func (s *CoordinatorServer) RequestInference(
	ctx context.Context,
	req *pb.InferenceRequest,
) (*pb.InferenceResponse, error) {
	start := time.Now()

	s.logger.WithFields(logrus.Fields{
		"client_id":  req.ClientId,
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
	}).Info("Inference request received")

	// Validate request
	if req.ClientId == "" {
		return nil, status.Error(codes.InvalidArgument, "client_id is required")
	}
	if req.Model == "" {
		return nil, status.Error(codes.InvalidArgument, "model is required")
	}
	if req.Prompt == "" {
		return nil, status.Error(codes.InvalidArgument, "prompt is required")
	}

	// Select optimal agent
	agent, err := s.scheduler.SelectAgent(req.Model)
	if err != nil {
		s.logger.WithError(err).Error("Agent selection failed")
		return nil, status.Error(codes.Unavailable, "no agents available")
	}

	// Generate JWT session token
	token, err := s.generateSessionToken(agent.ID, req.ClientId, req.Model)
	if err != nil {
		s.logger.WithError(err).Error("Failed to generate session token")
		return nil, status.Error(codes.Internal, "token generation failed")
	}

	// Create job ID for tracking
	jobID := uuid.New().String()

	schedulingLatency := time.Since(start).Milliseconds()

	s.logger.WithFields(logrus.Fields{
		"agent_id":       agent.ID,
		"agent_endpoint": agent.Endpoint,
		"job_id":         jobID,
		"session_token":  token.ID,
		"scheduling_ms":  schedulingLatency,
	}).Info("Inference request routed")

	return &pb.InferenceResponse{
		AgentEndpoint:  agent.Endpoint,
		SessionToken:   token.JWTToken, // Return the JWT token instead of ID
		ExpiresAt:      token.ExpiresAt.Unix(),
		JobId:          jobID,
		EstimatedRttMs: agent.RTTMs,
		AgentId:        agent.ID,
	}, nil
}

// RegisterAgent handles agent registration
func (s *CoordinatorServer) RegisterAgent(
	ctx context.Context,
	req *pb.AgentRegistration,
) (*pb.RegistrationResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"agent_id":         req.AgentId,
		"endpoint":         req.Endpoint,
		"total_vram_mb":    req.TotalVramMb,
		"max_jobs":         req.MaxJobs,
		"supported_models": req.SupportedModels,
	}).Info("Agent registration request")

	// Validate registration
	if req.AgentId == "" {
		return nil, status.Error(codes.InvalidArgument, "agent_id is required")
	}
	if req.Endpoint == "" {
		return nil, status.Error(codes.InvalidArgument, "endpoint is required")
	}

	// Create agent info
	agent := &registry.AgentInfo{
		ID:              req.AgentId,
		Endpoint:        req.Endpoint,
		TotalVRAMMB:     req.TotalVramMb,
		TotalRAMMB:      req.TotalRamMb,
		MaxJobs:         req.MaxJobs,
		SupportedModels: req.SupportedModels,
		Capabilities:    req.Capabilities,
		FreeVRAMMB:      req.TotalVramMb, // Initially assume all VRAM is free
		FreeRAMMB:       req.TotalRamMb,
	}

	// Register the agent
	if err := s.registry.Register(agent); err != nil {
		s.logger.WithError(err).Error("Agent registration failed")
		return nil, status.Error(codes.Internal, "registration failed")
	}

	return &pb.RegistrationResponse{
		Status:                   "success",
		Message:                  "Agent registered successfully",
		HeartbeatIntervalSeconds: int32(s.config.HeartbeatTimeout.Seconds() / 2),
	}, nil
}

// ReportHealth handles bidirectional health streaming
func (s *CoordinatorServer) ReportHealth(
	stream pb.CoordinatorService_ReportHealthServer,
) error {
	for {
		healthUpdate, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			s.logger.WithError(err).Error("Health stream error")
			return err
		}

		// Update agent health
		stats := &registry.HealthStats{
			AgentID:     healthUpdate.AgentId,
			FreeVRAMMB:  healthUpdate.FreeVramMb,
			FreeRAMMB:   healthUpdate.FreeRamMb,
			RunningJobs: healthUpdate.RunningJobs,
			QueuedJobs:  healthUpdate.QueuedJobs,
			CPUPercent:  healthUpdate.CpuPercent,
			RTTMs:       healthUpdate.RttMs,
			Timestamp:   healthUpdate.Timestamp,
		}

		if err := s.registry.UpdateHealth(healthUpdate.AgentId, stats); err != nil {
			s.logger.WithError(err).Warn("Failed to update agent health")

			// Send error ack
			ack := &pb.HealthAck{
				AgentId: healthUpdate.AgentId,
				Status:  "error",
				Message: err.Error(),
			}
			if sendErr := stream.Send(ack); sendErr != nil {
				return sendErr
			}
			continue
		}

		// Send success ack
		ack := &pb.HealthAck{
			AgentId: healthUpdate.AgentId,
			Status:  "ok",
			Message: "health updated",
		}
		if err := stream.Send(ack); err != nil {
			return err
		}
	}
}

// QuerySystemStatus returns system status information
func (s *CoordinatorServer) QuerySystemStatus(
	ctx context.Context,
	req *pb.StatusRequest,
) (*pb.SystemStatus, error) {
	total, healthy := s.registry.GetStats()

	status := &pb.SystemStatus{
		TotalAgents:   int32(total),
		HealthyAgents: int32(healthy),
		Uptime:        "unknown", // TODO: Track uptime
	}

	if req.IncludeAgents {
		agents := s.registry.ListAllAgents()
		status.Agents = make([]*pb.AgentStatus, len(agents))

		for i, agent := range agents {
			status.Agents[i] = &pb.AgentStatus{
				AgentId:       agent.ID,
				Status:        string(agent.Status),
				FreeVramMb:    agent.FreeVRAMMB,
				RunningJobs:   agent.RunningJobs,
				LastHeartbeat: agent.LastHeartbeat.Unix(),
			}
		}
	}

	return status, nil
}

// GetPublicKey handles public key distribution to agents for JWT validation
func (s *CoordinatorServer) GetPublicKey(
	ctx context.Context,
	req *pb.PublicKeyRequest,
) (*pb.PublicKeyResponse, error) {
	publicKeyPEM, err := s.jwtManager.GetPublicKeyPEM()
	if err != nil {
		s.logger.WithError(err).Error("Failed to get public key PEM")
		return nil, status.Error(codes.Internal, "failed to get public key")
	}

	s.logger.Debug("Public key requested by agent")

	return &pb.PublicKeyResponse{
		PublicKeyPem: publicKeyPEM,
		Algorithm:    "RS256",
		Issuer:       "titancompute-coordinator",
	}, nil
}

// generateSessionToken creates a new JWT session token
func (s *CoordinatorServer) generateSessionToken(agentID, clientID, model string) (*SessionToken, error) {
	now := time.Now()

	// Generate JWT token
	jwtToken, err := s.jwtManager.GenerateToken(agentID, clientID, model, s.config.TokenTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT token: %w", err)
	}

	token := &SessionToken{
		ID:        uuid.New().String(),
		AgentID:   agentID,
		ClientID:  clientID,
		Model:     model,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.config.TokenTTL),
		JWTToken:  jwtToken,
	}

	s.tokens[token.ID] = token

	// Clean up expired tokens periodically
	go s.cleanupExpiredTokens()

	return token, nil
}

// cleanupExpiredTokens removes expired tokens (basic implementation for M1)
func (s *CoordinatorServer) cleanupExpiredTokens() {
	time.Sleep(time.Minute) // Run cleanup after a minute

	now := time.Now()
	expired := make([]string, 0)

	for tokenID, token := range s.tokens {
		if now.After(token.ExpiresAt) {
			expired = append(expired, tokenID)
		}
	}

	for _, tokenID := range expired {
		delete(s.tokens, tokenID)
	}

	if len(expired) > 0 {
		s.logger.WithField("expired_tokens", len(expired)).Debug("Cleaned up expired tokens")
	}
}

// ValidateToken validates a JWT token (for internal use or debugging)
func (s *CoordinatorServer) ValidateToken(tokenString string) (*JWTClaims, error) {
	return s.jwtManager.ValidateToken(tokenString)
}
