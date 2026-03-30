package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/davidwhittington/habanero-agent/internal/config"
	"github.com/davidwhittington/habanero-agent/internal/engine"
	"github.com/davidwhittington/habanero-agent/internal/fleet"
	"github.com/davidwhittington/habanero-agent/internal/mesh"
	"github.com/davidwhittington/habanero-agent/internal/monitor"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		configPath  = flag.String("config", "/etc/habanero/agent.yml", "path to configuration file")
		showVersion = flag.Bool("version", false, "print version and exit")
		checkConfig = flag.Bool("check", false, "validate configuration and exit")
		runOnce     = flag.Bool("once", false, "run diagnostic once and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("habanero-agent %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	if *checkConfig {
		fmt.Println("configuration is valid")
		os.Exit(0)
	}

	// Set up structured JSON logging.
	logLevel := parseLogLevel(cfg.Logging.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("starting habanero-agent",
		"version", version,
		"agent_id", cfg.Agent.ID,
		"agent_name", cfg.Agent.Name,
		"site", cfg.Agent.Site,
	)

	// Initialize diagnostic engine.
	diagEngine := engine.New(cfg)

	// Single-run mode: run diagnostics once and exit.
	if *runOnce {
		result := diagEngine.Run()
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
		os.Exit(0)
	}

	// Set up graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	var wg sync.WaitGroup

	// Initialize fleet client.
	fleetClient := fleet.NewClient(cfg.Fleet.Endpoint, cfg.Fleet.APIKey)

	// Register agent with fleet.
	if err := fleetClient.Register(ctx, cfg.Agent); err != nil {
		slog.Warn("fleet registration failed, will retry", "error", err)
	}

	// Start continuous monitoring loop.
	mon := monitor.New(cfg)
	wg.Add(1)
	go func() {
		defer wg.Done()
		mon.Run(ctx)
	}()

	// Start diagnostic engine on interval.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runDiagnosticLoop(ctx, diagEngine, fleetClient, cfg)
	}()

	// Start mesh probing if peers are available.
	meshMgr := mesh.NewManager(cfg, fleetClient)
	wg.Add(1)
	go func() {
		defer wg.Done()
		meshMgr.Run(ctx)
	}()

	// Start fleet reporter.
	wg.Add(1)
	go func() {
		defer wg.Done()
		runFleetReporter(ctx, fleetClient, mon, cfg)
	}()

	slog.Info("all subsystems started, waiting for signals")

	// Wait for shutdown signal.
	<-sigCh
	slog.Info("shutdown signal received, stopping gracefully")
	cancel()

	// Give goroutines time to finish.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("all subsystems stopped cleanly")
	case <-time.After(10 * time.Second):
		slog.Warn("graceful shutdown timed out, forcing exit")
	}
}

func runDiagnosticLoop(ctx context.Context, eng *engine.DiagnosticEngine, fc *fleet.Client, cfg *config.Config) {
	interval := time.Duration(cfg.Fleet.ReportInterval) * time.Second
	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			slog.Info("running scheduled diagnostic")
			result := eng.Run()

			if err := fc.ReportDiagnostic(ctx, result); err != nil {
				slog.Error("failed to report diagnostic", "error", err)
			}
		}
	}
}

func runFleetReporter(ctx context.Context, fc *fleet.Client, mon *monitor.Monitor, cfg *config.Config) {
	interval := time.Duration(cfg.Fleet.ReportInterval) * time.Second
	if interval == 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			metrics := mon.Snapshot()
			if err := fc.ReportMetrics(ctx, metrics); err != nil {
				slog.Error("failed to report metrics", "error", err)
			}
		}
	}
}

func parseLogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
