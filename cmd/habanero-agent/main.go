package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/davidwhittington/habanero-agent/internal/activation"
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
	activateCode := flag.String("activate", "", "Activate with code (e.g., HAB-XXXX-XXXX-XXXX-XXXX)")
	setupMode := flag.Bool("setup", false, "Interactive setup: display QR code and wait for activation")
	dashboardURL := flag.String("dashboard", "https://dashboard.habanero.tools", "Dashboard URL for activation")
	flag.Parse()

	if *showVersion {
		fmt.Printf("habanero-agent %s\n", version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Interactive setup mode: generate code, show QR, wait for activation
	if *setupMode {
		hostname, _ := os.Hostname()
		apiEndpoint := "https://api.ipvegan.com/api/v1"

		// Request activation code from API
		code, err := activation.GenerateCode(apiEndpoint, hostname)
		if err != nil {
			// Generate a local code and display instructions
			code = fmt.Sprintf("HAB-%s", "SETUP")
			fmt.Printf("Could not auto-generate code: %v\n", err)
			fmt.Println("Generate one from the dashboard and run: habanero-agent --activate HAB-XXXX-XXXX-XXXX-XXXX")
			os.Exit(1)
		}

		activation.Display(code, *dashboardURL)

		// Wait for activation
		result, err := activation.WaitForActivation(code, apiEndpoint, logger)
		if err != nil {
			fmt.Printf("Activation failed: %v\n", err)
			os.Exit(1)
		}

		// Write config with the received credentials
		configContent := fmt.Sprintf(`agent:
  id: %s
  name: %s
  site: %s

fleet:
  endpoint: https://api.ipvegan.com/api/v1
  api_key: %s
  report_interval: 60

monitoring:
  continuous: true

logging:
  level: info
`, hostname, hostname, result.SiteName, result.ConsultantKey)

		os.MkdirAll("/etc/habanero", 0755)
		if err := os.WriteFile("/etc/habanero/agent.yml", []byte(configContent), 0600); err != nil {
			fmt.Printf("Failed to write config: %v\n", err)
			fmt.Println("Config content:")
			fmt.Println(configContent)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ Activated! Connected to %s's fleet at site '%s'\n", result.ConsultantName, result.SiteName)
		fmt.Println("  Config written to /etc/habanero/agent.yml")
		fmt.Println("  Start the agent: systemctl enable --now habanero-agent")
		os.Exit(0)
	}

	// Direct activation with a code
	if *activateCode != "" {
		hostname, _ := os.Hostname()
		resp, err := http.Post(
			"https://api.ipvegan.com/api/v1/fleet/activate",
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"code":"%s","device_id":"%s","hostname":"%s","os_version":"%s/%s","agent_version":"%s"}`,
				*activateCode, hostname, hostname, "linux", "amd64", version)),
		)
		if err != nil {
			fmt.Printf("Activation failed: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		var result struct {
			OK         bool   `json:"ok"`
			Message    string `json:"message"`
			SiteName   string `json:"site_name"`
			Consultant string `json:"consultant"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if !result.OK {
			fmt.Printf("Activation failed: %s\n", result.Message)
			os.Exit(1)
		}

		fmt.Printf("\n  ✓ %s\n", result.Message)
		fmt.Printf("  Site: %s\n", result.SiteName)
		fmt.Println("  Update /etc/habanero/agent.yml with the provided API key,")
		fmt.Println("  then: systemctl restart habanero-agent")
		os.Exit(0)
	}
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
