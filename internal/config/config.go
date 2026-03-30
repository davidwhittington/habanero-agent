package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for habanero-agent.
type Config struct {
	Agent        AgentConfig        `yaml:"agent"`
	Fleet        FleetConfig        `yaml:"fleet"`
	Monitoring   MonitoringConfig   `yaml:"monitoring"`
	Alerts       AlertsConfig       `yaml:"alerts"`
	Integrations IntegrationsConfig `yaml:"integrations"`
	Logging      LoggingConfig      `yaml:"logging"`
	Storage      StorageConfig      `yaml:"storage"`
}

type AgentConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Site string `yaml:"site"`
}

type FleetConfig struct {
	Endpoint       string `yaml:"endpoint"`
	APIKey         string `yaml:"api_key"`
	ConsultantID   string `yaml:"consultant_id"`
	ReportInterval int    `yaml:"report_interval"`
}

type MonitoringConfig struct {
	Continuous            bool                `yaml:"continuous"`
	PingTargets           []string            `yaml:"ping_targets"`
	PingInterval          int                 `yaml:"ping_interval"`
	DNSTargets            []string            `yaml:"dns_targets"`
	DNSInterval           int                 `yaml:"dns_interval"`
	BandwidthTestInterval int                 `yaml:"bandwidth_test_interval"`
	PacketCapture         PacketCaptureConfig `yaml:"packet_capture"`
	SNMP                  SNMPConfig          `yaml:"snmp"`
}

type PacketCaptureConfig struct {
	Enabled      bool   `yaml:"enabled"`
	Interface    string `yaml:"interface"`
	Duration     int    `yaml:"duration"`
	Interval     int    `yaml:"interval"`
	MaxStorage   string `yaml:"max_storage"`
	AnalysisOnly bool   `yaml:"analysis_only"`
}

type SNMPConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Community    string   `yaml:"community"`
	Targets      []string `yaml:"targets"`
	PollInterval int      `yaml:"poll_interval"`
}

type AlertsConfig struct {
	Rules []AlertRule `yaml:"rules"`
}

type AlertRule struct {
	Name        string      `yaml:"name"`
	Metric      string      `yaml:"metric"`
	Operator    string      `yaml:"operator"`
	Threshold   interface{} `yaml:"threshold"`
	Consecutive int         `yaml:"consecutive"`
	Severity    string      `yaml:"severity"`
	Actions     []string    `yaml:"actions"`
}

type IntegrationsConfig struct {
	PagerDuty  PagerDutyConfig  `yaml:"pagerduty"`
	ServiceNow ServiceNowConfig `yaml:"servicenow"`
	Jira       JiraConfig       `yaml:"jira"`
	Slack      SlackConfig      `yaml:"slack"`
	Zendesk    ZendeskConfig    `yaml:"zendesk"`
}

type PagerDutyConfig struct {
	Enabled     bool              `yaml:"enabled"`
	RoutingKey  string            `yaml:"routing_key"`
	SeverityMap map[string]string `yaml:"severity_map"`
}

type ServiceNowConfig struct {
	Enabled         bool              `yaml:"enabled"`
	Instance        string            `yaml:"instance"`
	Auth            ServiceNowAuth    `yaml:"auth"`
	Table           string            `yaml:"table"`
	AssignmentGroup string            `yaml:"assignment_group"`
	CallerID        string            `yaml:"caller_id"`
	Category        string            `yaml:"category"`
	AutoResolve     bool              `yaml:"auto_resolve"`
	FieldMap        map[string]string `yaml:"field_map"`
}

type ServiceNowAuth struct {
	Username    string `yaml:"username"`
	PasswordEnv string `yaml:"password_env"`
}

type JiraConfig struct {
	Enabled     bool   `yaml:"enabled"`
	URL         string `yaml:"url"`
	Project     string `yaml:"project"`
	IssueType   string `yaml:"issue_type"`
	APITokenEnv string `yaml:"api_token_env"`
}

type SlackConfig struct {
	Enabled           bool   `yaml:"enabled"`
	WebhookURLEnv     string `yaml:"webhook_url_env"`
	Channel           string `yaml:"channel"`
	MentionOnCritical string `yaml:"mention_on_critical"`
}

type ZendeskConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Subdomain   string `yaml:"subdomain"`
	APITokenEnv string `yaml:"api_token_env"`
	GroupID     string `yaml:"group_id"`
}

type LoggingConfig struct {
	Level    string `yaml:"level"`
	File     string `yaml:"file"`
	MaxSize  string `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
}

type StorageConfig struct {
	DataDir            string `yaml:"data_dir"`
	HistoryRetention   string `yaml:"history_retention"`
	MetricsRetention   string `yaml:"metrics_retention"`
	PcapRetention      string `yaml:"pcap_retention"`
}

// envVarPattern matches ${VAR} and ${VAR:-default}.
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// Load reads and parses a YAML config file, performing environment variable
// interpolation on the raw YAML before unmarshalling.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Interpolate environment variables.
	expanded := interpolateEnvVars(string(data))

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// interpolateEnvVars replaces ${VAR} and ${VAR:-default} patterns with
// their environment variable values.
func interpolateEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		groups := envVarPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}

		varName := groups[1]
		defaultVal := ""
		if len(groups) >= 3 {
			defaultVal = groups[2]
		}

		if val, ok := os.LookupEnv(varName); ok {
			return val
		}
		return defaultVal
	})
}

// applyDefaults fills in zero values with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.Agent.ID == "" || cfg.Agent.ID == "auto" {
		cfg.Agent.ID = generateAgentID()
	}

	if cfg.Fleet.ReportInterval == 0 {
		cfg.Fleet.ReportInterval = 300
	}

	if cfg.Monitoring.PingInterval == 0 {
		cfg.Monitoring.PingInterval = 10
	}

	if cfg.Monitoring.DNSInterval == 0 {
		cfg.Monitoring.DNSInterval = 30
	}

	if cfg.Monitoring.BandwidthTestInterval == 0 {
		cfg.Monitoring.BandwidthTestInterval = 3600
	}

	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}

	if cfg.Logging.File == "" {
		cfg.Logging.File = "/var/log/habanero/agent.log"
	}

	if cfg.Logging.MaxFiles == 0 {
		cfg.Logging.MaxFiles = 10
	}

	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "/var/lib/habanero"
	}

	if cfg.Storage.HistoryRetention == "" {
		cfg.Storage.HistoryRetention = "30d"
	}

	if cfg.Storage.MetricsRetention == "" {
		cfg.Storage.MetricsRetention = "7d"
	}
}

func validate(cfg *Config) error {
	if cfg.Agent.Name == "" {
		return fmt.Errorf("agent.name is required")
	}

	if cfg.Agent.Site == "" {
		return fmt.Errorf("agent.site is required")
	}

	if cfg.Fleet.Endpoint == "" {
		return fmt.Errorf("fleet.endpoint is required")
	}

	if cfg.Fleet.APIKey == "" {
		return fmt.Errorf("fleet.api_key is required")
	}

	// Validate alert rules.
	for i, rule := range cfg.Alerts.Rules {
		if rule.Name == "" {
			return fmt.Errorf("alerts.rules[%d].name is required", i)
		}
		if rule.Metric == "" {
			return fmt.Errorf("alerts.rules[%d].metric is required", i)
		}
		validOps := map[string]bool{"eq": true, "gt": true, "lt": true, "gte": true, "lte": true, "ne": true}
		if !validOps[rule.Operator] {
			return fmt.Errorf("alerts.rules[%d].operator %q is invalid", i, rule.Operator)
		}
	}

	return nil
}

// generateAgentID creates a simple unique identifier based on hostname and timestamp.
func generateAgentID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("hab-%s-%d", strings.ToLower(hostname), os.Getpid())
}
