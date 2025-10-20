package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the coordinator configuration
type Config struct {
	Port             int
	HTTPPort         int
	HeartbeatTimeout time.Duration
	TokenTTL         time.Duration
	CleanupInterval  time.Duration
}

// Load returns the coordinator configuration from environment variables
func Load() *Config {
	cfg := &Config{
		Port:             50051,
		HTTPPort:         8080,
		HeartbeatTimeout: 30 * time.Second,
		TokenTTL:         60 * time.Second,
		CleanupInterval:  60 * time.Second,
	}

	// Override with environment variables if set
	if port := os.Getenv("COORDINATOR_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.Port = p
		}
	}

	if httpPort := os.Getenv("COORDINATOR_HTTP_PORT"); httpPort != "" {
		if p, err := strconv.Atoi(httpPort); err == nil {
			cfg.HTTPPort = p
		}
	}

	if timeout := os.Getenv("HEARTBEAT_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			cfg.HeartbeatTimeout = d
		}
	}

	if ttl := os.Getenv("TOKEN_TTL"); ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			cfg.TokenTTL = d
		}
	}

	return cfg
}
