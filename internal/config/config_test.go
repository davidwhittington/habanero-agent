package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInterpolateEnvVars(t *testing.T) {
	os.Setenv("TEST_HAB_KEY", "secret-key-123")
	defer os.Unsetenv("TEST_HAB_KEY")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple var",
			input:    "api_key: ${TEST_HAB_KEY}",
			expected: "api_key: secret-key-123",
		},
		{
			name:     "var with default, var set",
			input:    "api_key: ${TEST_HAB_KEY:-fallback}",
			expected: "api_key: secret-key-123",
		},
		{
			name:     "var with default, var unset",
			input:    "api_key: ${UNSET_VAR:-fallback-value}",
			expected: "api_key: fallback-value",
		},
		{
			name:     "unset var without default",
			input:    "api_key: ${UNSET_VAR}",
			expected: "api_key: ",
		},
		{
			name:     "no vars",
			input:    "api_key: plain-value",
			expected: "api_key: plain-value",
		},
		{
			name:     "multiple vars",
			input:    "key: ${TEST_HAB_KEY} and ${UNSET_VAR:-other}",
			expected: "key: secret-key-123 and other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolateEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	yaml := `
agent:
  id: test-agent
  name: test-host
  site: test-site
fleet:
  endpoint: https://api.example.com/fleet
  api_key: HAB-TEST-1234
  report_interval: 60
monitoring:
  continuous: true
  ping_targets:
    - 1.1.1.1
    - 8.8.8.8
  ping_interval: 5
  dns_targets:
    - example.com
  dns_interval: 15
alerts:
  rules:
    - name: "Test alert"
      metric: gateway_rtt
      operator: "gt"
      threshold: 100
      consecutive: 3
      severity: high
      actions: [webhook]
logging:
  level: debug
`

	dir := t.TempDir()
	path := filepath.Join(dir, "agent.yml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Agent.ID != "test-agent" {
		t.Errorf("agent.id = %q, want %q", cfg.Agent.ID, "test-agent")
	}
	if cfg.Agent.Name != "test-host" {
		t.Errorf("agent.name = %q, want %q", cfg.Agent.Name, "test-host")
	}
	if cfg.Monitoring.PingInterval != 5 {
		t.Errorf("monitoring.ping_interval = %d, want 5", cfg.Monitoring.PingInterval)
	}
	if cfg.Fleet.ReportInterval != 60 {
		t.Errorf("fleet.report_interval = %d, want 60", cfg.Fleet.ReportInterval)
	}
	if len(cfg.Alerts.Rules) != 1 {
		t.Errorf("alerts.rules length = %d, want 1", len(cfg.Alerts.Rules))
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("logging.level = %q, want %q", cfg.Logging.Level, "debug")
	}
	// Defaults should be applied.
	if cfg.Storage.DataDir != "/var/lib/habanero" {
		t.Errorf("storage.data_dir = %q, want default", cfg.Storage.DataDir)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "missing agent name",
			yaml: `
agent:
  site: test
fleet:
  endpoint: https://example.com
  api_key: test
`,
		},
		{
			name: "missing fleet endpoint",
			yaml: `
agent:
  name: test
  site: test
fleet:
  api_key: test
`,
		},
		{
			name: "invalid alert operator",
			yaml: `
agent:
  name: test
  site: test
fleet:
  endpoint: https://example.com
  api_key: test
alerts:
  rules:
    - name: "Bad rule"
      metric: test
      operator: "invalid"
      threshold: 1
      severity: low
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "agent.yml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := Load(path)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}
