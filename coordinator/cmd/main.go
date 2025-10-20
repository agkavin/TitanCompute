package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
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
	grpcListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.WithError(err).Fatal("Failed to listen on gRPC port")
	}

	grpcServer := grpc.NewServer()

	// Register gRPC services
	server.RegisterCoordinatorService(grpcServer, coordinatorServer)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Setup REST API server
	restHandler := server.NewRESTHandler(coordinatorServer, logger)
	httpRouter := restHandler.SetupRoutes()

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      httpRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start cleanup routine
	go agentRegistry.StartCleanup()

	// Start gRPC server in goroutine
	go func() {
		logger.WithField("address", grpcListener.Addr().String()).Info("🎯 gRPC server listening")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.WithError(err).Fatal("Failed to serve gRPC")
		}
	}()

	// Start HTTP server in goroutine
	go func() {
		logger.WithField("address", httpServer.Addr).Info("🌐 REST API server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Failed to serve HTTP")
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	logger.Info("🛑 Shutting down coordinator...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop accepting new connections
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.WithError(err).Warn("HTTP server shutdown error")
	}

	// Shutdown gRPC server
	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("✅ Graceful shutdown completed")
	case <-ctx.Done():
		logger.Warn("⚠️ Force stopping server after timeout")
		grpcServer.Stop()
	}
}
