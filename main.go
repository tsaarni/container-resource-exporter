package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var configPath = flag.String("config", "config.yaml", "Path to configuration file")

func setupLogging(level string) {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	case "none":
		logLevel = slog.Level(999)
	default:
		slog.Warn("Unknown log level, defaulting to info", "log_level", level)
		logLevel = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))
}

func main() {
	flag.Parse()

	config, err := LoadConfig(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err, "path", *configPath)
		os.Exit(1)
	}

	setupLogging(config.LogLevel)

	slog.Info("Starting container-resource-exporter",
		"config", *configPath,
		"address", config.Server.Address,
		"scrape_interval", config.ScrapeInterval,
		"log_level", config.LogLevel,
	)

	kubeClient, err := NewKubernetesClient(config)
	if err != nil {
		slog.Error("Failed to create Kubernetes client", "error", err)
		os.Exit(1)
	}

	// Start collector,
	collector := NewCollector(config, kubeClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go collector.Start(ctx)

	// Setup HTTP server.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/metrics", http.StatusFound)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    config.Server.Address,
		Handler: mux,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		slog.Info("Received shutdown signal")
		cancel()
		if err := server.Shutdown(context.Background()); err != nil {
			slog.Error("Server shutdown error", "error", err)
		}
	}()

	slog.Info("HTTP server listening", "address", config.Server.Address)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Exporter stopped")
}
