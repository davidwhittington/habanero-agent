package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/davidwhittington/habanero-agent/internal/engine"
	"github.com/davidwhittington/habanero-agent/internal/fleet"
)

type Monitor struct {
	eng      *engine.DiagnosticEngine
	fleet    *fleet.Client
	interval time.Duration
	logger   *slog.Logger
}

func New(eng *engine.DiagnosticEngine, fleetClient *fleet.Client, interval time.Duration, logger *slog.Logger) *Monitor {
	return &Monitor{eng: eng, fleet: fleetClient, interval: interval, logger: logger}
}

func (m *Monitor) Run(ctx context.Context) {
	m.logger.Info("monitor started", "interval", m.interval)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Run immediately on start
	m.runOnce()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor stopped")
			return
		case <-ticker.C:
			m.runOnce()
		}
	}
}

func (m *Monitor) runOnce() {
	result, err := m.eng.Run()
	if err != nil {
		m.logger.Error("diagnostic failed", "error", err)
		return
	}

	if m.fleet != nil {
		if err := m.fleet.ReportDiagnostic(result); err != nil {
			m.logger.Warn("failed to report to fleet", "error", err)
		}
	}
}
