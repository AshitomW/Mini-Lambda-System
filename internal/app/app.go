// Package app handles dependency wiring, server initialization, and graceful shutdown orchestration.
package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"AshitomW/mini-lambda/internal/config"
	"AshitomW/mini-lambda/internal/handler"
	"AshitomW/mini-lambda/internal/metrics"
	"AshitomW/mini-lambda/internal/repository"
	"AshitomW/mini-lambda/internal/runner"
	"AshitomW/mini-lambda/internal/service"
)

// Run boots up the application server, initializes components, and oversees graceful termination.
func Run() error {
	cfg := config.Load()

	funcRepo, err := repository.NewFileFunctionRepository(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("failed to initialize function repository: %w", err)
	}

	invRepo := repository.NewMemoryInvocationRepository(0)

	dockerRunner, err := runner.NewDockerRunner(cfg)
	if err != nil {
		log.Printf("Warning: Docker daemon connection failed: %v. Execution will return errors until daemon is available.", err)
	} else {
		defer dockerRunner.Close()
	}

	promMetrics := metrics.NewPrometheusMetrics()

	funcService := service.NewFunctionService(funcRepo)
	invService := service.NewInvocationService(funcRepo, invRepo, dockerRunner, promMetrics, cfg)
	defer invService.Close()

	imgService := service.NewImageService(dockerRunner)

	h := handler.NewHandler(funcService, invService, imgService, promMetrics)
	r := handler.NewRouter(h)

	addr := cfg.Port
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("Mini-Lambda server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdownSig:
		log.Printf("Received termination signal %s. Initiating graceful shutdown...", sig)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
		return fmt.Errorf("failed to gracefully shut down server: %w", err)
	}

	log.Println("Server gracefully stopped.")
	return nil
}
