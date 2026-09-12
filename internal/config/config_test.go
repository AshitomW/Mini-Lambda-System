package config_test

import (
	"os"
	"testing"
	"time"

	"AshitomW/mini-lambda/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	os.Clearenv()
	cfg := config.Load()

	if cfg.Port != "8300" {
		t.Errorf("expected port 8300, got %s", cfg.Port)
	}
	if cfg.DataDir != "function" {
		t.Errorf("expected dataDir function, got %s", cfg.DataDir)
	}
	if cfg.MaxConcurrentInvocations != 20 {
		t.Errorf("expected 20 concurrent invocations, got %d", cfg.MaxConcurrentInvocations)
	}
	if cfg.DefaultTimeout != 120*time.Second {
		t.Errorf("expected 120s timeout, got %v", cfg.DefaultTimeout)
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("PORT", "9000")
	os.Setenv("DATA_DIR", "testdata")
	os.Setenv("MAX_CONCURRENT_INVOCATIONS", "50")
	os.Setenv("DEFAULT_TIMEOUT_SEC", "60")
	os.Setenv("MAX_TIMEOUT_SEC", "300")
	os.Setenv("SHUTDOWN_TIMEOUT_SEC", "5")
	os.Setenv("CONTAINER_MEMORY_LIMIT_MB", "512")
	os.Setenv("CONTAINER_CPU_SHARES", "2048")
	defer os.Clearenv()

	cfg := config.Load()

	if cfg.Port != "9000" {
		t.Errorf("expected port 9000, got %s", cfg.Port)
	}
	if cfg.DataDir != "testdata" {
		t.Errorf("expected dataDir testdata, got %s", cfg.DataDir)
	}
	if cfg.MaxConcurrentInvocations != 50 {
		t.Errorf("expected 50 concurrent invocations, got %d", cfg.MaxConcurrentInvocations)
	}
	if cfg.DefaultTimeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", cfg.DefaultTimeout)
	}
	if cfg.MaxTimeout != 300*time.Second {
		t.Errorf("expected 300s max timeout, got %v", cfg.MaxTimeout)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("expected 5s shutdown timeout, got %v", cfg.ShutdownTimeout)
	}
	if cfg.MemoryLimitBytes != 512*1024*1024 {
		t.Errorf("expected 512MB memory limit, got %d", cfg.MemoryLimitBytes)
	}
	if cfg.CPUShares != 2048 {
		t.Errorf("expected 2048 cpu shares, got %d", cfg.CPUShares)
	}
}
