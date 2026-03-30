package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/davidwhittington/habanero-agent/internal/config"
	"github.com/davidwhittington/habanero-agent/internal/engine"
	"github.com/davidwhittington/habanero-agent/internal/fleet"
	"github.com/davidwhittington/habanero-agent/internal/mesh"
	"github.com/davidwhittington/habanero-agent/internal/monitor"
)

var version = "0.1.0"

func main() {
	configPath := flag.String("config", "/etc/habanero/agent.yml", "Path to config file")
	showVersion := flag.Bool("version", false, "Show version")
	runOnce := flag.Bool("once", false, "Run diagnostic once and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("habanero-agent %s\n", version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load config", "path", *configPath, "error", err)
		os.Exit(1)
	}
	logger.Info("config loaded", "agent_id", cfg.Agent.ID, "agent_name", cfg.Agent.Name)

	eng := engine.New(logger)

	if *runOnce {
		result, err := eng.Run()
		if err != nil {
			logger.Error("diagnostic failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf(`{"status":"%s","chain":"%s","findings":%d,"gateway":"%s","can_route":%t,"dns_works":%t}`+"\n",
			result.Status, result.ChainTrace, len(result.Findings), result.Gateway, result.CanRoute, result.DNSWorks)
		os.Exit(0)
	}

	var fleetClient *fleet.Client
	if cfg.Fleet.Endpoint != "" && cfg.Fleet.APIKey != "" {
		fleetClient = fleet.NewClient(cfg.Fleet.Endpoint, cfg.Fleet.APIKey, cfg.Agent.ID, logger)
		if err := fleetClient.Register(); err != nil {
			logger.Warn("fleet registration failed", "error", err)
		} else {
			logger.Info("registered with fleet", "endpoint", cfg.Fleet.Endpoint)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	interval := time.Duration(cfg.Fleet.ReportInterval) * time.Second
	if interval == 0 {
		interval = 300 * time.Second
	}

	mon := monitor.New(eng, fleetClient, interval, logger)
	go mon.Run(ctx)

	if fleetClient != nil {
		meshMgr := mesh.NewManager(fleetClient, logger)
		go meshMgr.Run(ctx)
	}

	logger.Info("habanero-agent started", "version", version, "interval", interval)

	sig := <-sigCh
	logger.Info("shutting down", "signal", sig.String())
	cancel()
	time.Sleep(time.Second)
}
