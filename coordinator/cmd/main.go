package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/titancompute/coordinator/pkg/config"
	"github.com/titancompute/coordinator/pkg/registry"
	"github.com/titancompute/coordinator/pkg/scheduler"
	"github.com/titancompute/coordinator/pkg/server"
)

func main() {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	logger.Info("🚀 Starting TitanCompute Coordinator")

	// Load configuration
	cfg := config.Load()
	logger.WithFields(logrus.Fields{
		"port":              cfg.Port,
		"heartbeat_timeout": cfg.HeartbeatTimeout,
		"token_ttl":         cfg.TokenTTL,
	}).Info("Configuration loaded")

	// Initialize components
	agentRegistry := registry.NewInMemoryRegistry(cfg.HeartbeatTimeout, logger)

	// Use MCDA scheduler for M2 (memory-aware multi-criteria decision analysis)
	agentScheduler := scheduler.NewMCDAScheduler(agentRegistry, logger)
	logger.Info("🧠 Using MCDA scheduler for memory-aware agent selection")

	coordinatorServer, err := server.NewCoordinatorServer(agentRegistry, agentScheduler, cfg, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to create coordinator server")
	}

	// Setup gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.WithError(err).Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()

	// Register services
	server.RegisterCoordinatorService(grpcServer, coordinatorServer)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Start cleanup routine
	go agentRegistry.StartCleanup()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("🛑 Shutting down coordinator...")

		// Stop accepting new connections
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

		// Graceful stop with timeout
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("✅ Graceful shutdown completed")
		case <-time.After(30 * time.Second):
			logger.Warn("⚠️ Force stopping server after timeout")
			grpcServer.Stop()
		}

		os.Exit(0)
	}()

	// Start server
	logger.WithField("address", lis.Addr().String()).Info("🎯 Coordinator server listening")
	if err := grpcServer.Serve(lis); err != nil {
		logger.WithError(err).Fatal("Failed to serve")
	}
}
