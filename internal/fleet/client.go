package fleet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/davidwhittington/habanero-agent/internal/engine"
)

type Client struct {
	endpoint string
	apiKey   string
	agentID  string
	hostname string
	logger   *slog.Logger
	http     *http.Client
}

func NewClient(endpoint, apiKey, agentID string, logger *slog.Logger) *Client {
	hostname, _ := os.Hostname()
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		agentID:  agentID,
		hostname: hostname,
		logger:   logger,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) Register() error {
	body := map[string]any{
		"device_id":        c.agentID,
		"hostname":         c.hostname,
		"site_fingerprint": c.agentID,
		"tier":             "agent",
		"os_version":       runtime.GOOS + "/" + runtime.GOARCH,
		"agent_version":    "0.1.0",
	}
	return c.post("/api/fleet/register", body)
}

func (c *Client) ReportDiagnostic(result *engine.DiagnosticResult) error {
	body := map[string]any{
		"device_id":  c.agentID,
		"hostname":   c.hostname,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"diagnostic": result,
	}
	return c.post("/api/fleet/diagnostic", body)
}

func (c *Client) ReportMetrics(metrics map[string]any) error {
	metrics["device_id"] = c.agentID
	metrics["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	return c.post("/api/fleet/metrics", metrics)
}

func (c *Client) GetPeers() ([]map[string]any, error) {
	// TODO: GET /api/fleet/mesh/peers
	return nil, nil
}

func (c *Client) post(path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.endpoint+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		c.logger.Warn("fleet API call failed", "path", path, "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fleet API error: %d", resp.StatusCode)
	}

	c.logger.Info("fleet API call", "path", path, "status", resp.StatusCode)
	return nil
}
