package engine

import (
	"log/slog"
	"time"
)

type DiagnosticEngine struct {
	logger *slog.Logger
}

type DiagnosticResult struct {
	ChainTrace string   `json:"chain_trace"`
	Status     string   `json:"status"`
	Findings   []Finding `json:"findings"`
	Gateway    string   `json:"gateway"`
	IP         string   `json:"ip"`
	CanRoute   bool     `json:"can_route"`
	DNSWorks   bool     `json:"dns_works"`
}

type Finding struct {
	Severity    string `json:"severity"`
	Layer       string `json:"layer"`
	Description string `json:"description"`
}

func New(logger *slog.Logger) *DiagnosticEngine {
	return &DiagnosticEngine{logger: logger}
}

func (e *DiagnosticEngine) Run() (*DiagnosticResult, error) {
	e.logger.Info("running diagnostic chain")
	result := &DiagnosticResult{Status: "healthy"}

	gw := GetDefaultGateway()
	result.Gateway = gw
	if gw == "" {
		result.Status = "chain_broken"
		result.Findings = append(result.Findings, Finding{"CRITICAL", "L2", "No default gateway"})
		return result, nil
	}

	ping := Ping("1.1.1.1", 3, 5*time.Second)
	if ping.AvgRTT == 0 {
		result.Status = "chain_broken"
		result.Findings = append(result.Findings, Finding{"CRITICAL", "L3", "Cannot reach public internet"})
		return result, nil
	}
	result.CanRoute = true
	result.ChainTrace = "gateway_ok -> routable"

	dns, _, _ := DNSResolve("example.com", "1.1.1.1")
	if dns != "" {
		result.DNSWorks = true
		result.ChainTrace += " -> dns_ok"
	} else {
		result.Findings = append(result.Findings, Finding{"WARNING", "L3", "DNS resolution failed"})
	}

	return result, nil
}
