// Package config handles system configuration loaded from environment variables with safe defaults.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config encapsulates configuration parameters for the server and runtime.
type Config struct {
	Port                     string
	DataDir                  string
	MaxConcurrentInvocations int
	DefaultTimeout           time.Duration
	MaxTimeout               time.Duration
	ShutdownTimeout          time.Duration
	MemoryLimitBytes         int64
	CPUShares                int64
	TLSEnabled               bool
	TLSCertFile              string
	TLSKeyFile               string
	MTLSEnabled              bool
	MTLSClientCAFile         string
	AuthEnabled              bool
	AdminAPIKey              string
	InvokerAPIKey            string
	ContainerNetworkEnabled  bool
	MaxRetries               int
	WarmPoolEnabled          bool
	WarmPoolIdleTimeout      time.Duration
}

// Load reads configuration from environment variables, falling back to sensible defaults.
func Load() Config {
	cfg := Config{
		Port:                     getEnv("PORT", "8300"),
		DataDir:                  getEnv("DATA_DIR", "function"),
		MaxConcurrentInvocations: getEnvInt("MAX_CONCURRENT_INVOCATIONS", 20),
		DefaultTimeout:           time.Duration(getEnvInt("DEFAULT_TIMEOUT_SEC", 120)) * time.Second,
		MaxTimeout:               time.Duration(getEnvInt("MAX_TIMEOUT_SEC", 900)) * time.Second,
		ShutdownTimeout:          time.Duration(getEnvInt("SHUTDOWN_TIMEOUT_SEC", 10)) * time.Second,
		MemoryLimitBytes:         getEnvInt64("CONTAINER_MEMORY_LIMIT_MB", 256) * 1024 * 1024,
		CPUShares:                getEnvInt64("CONTAINER_CPU_SHARES", 1024),
		TLSEnabled:               getEnvBool("TLS_ENABLED", false),
		TLSCertFile:              getEnv("TLS_CERT_FILE", ""),
		TLSKeyFile:               getEnv("TLS_KEY_FILE", ""),
		MTLSEnabled:              getEnvBool("MTLS_ENABLED", false),
		MTLSClientCAFile:         getEnv("MTLS_CLIENT_CA_FILE", ""),
		AuthEnabled:              getEnvBool("AUTH_ENABLED", false),
		AdminAPIKey:              getEnv("ADMIN_API_KEY", ""),
		InvokerAPIKey:            getEnv("INVOKER_API_KEY", ""),
		ContainerNetworkEnabled:  getEnvBool("CONTAINER_NETWORK_ENABLED", false),
		MaxRetries:               getEnvInt("MAX_RETRIES", 3),
		WarmPoolEnabled:          getEnvBool("WARM_POOL_ENABLED", true),
		WarmPoolIdleTimeout:      time.Duration(getEnvInt("WARM_POOL_IDLE_TIMEOUT_SEC", 60)) * time.Second,
	}

	if cfg.MaxConcurrentInvocations <= 0 {
		cfg.MaxConcurrentInvocations = 20
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 120 * time.Second
	}
	if cfg.MaxTimeout < cfg.DefaultTimeout {
		cfg.MaxTimeout = cfg.DefaultTimeout
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.WarmPoolIdleTimeout <= 0 {
		cfg.WarmPoolIdleTimeout = 60 * time.Second
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if parsed, err := strconv.ParseBool(val); err == nil {
			return parsed
		}
	}
	return defaultVal
}
