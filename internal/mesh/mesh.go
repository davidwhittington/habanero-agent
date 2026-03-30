package mesh

import (
	"context"
	"log/slog"

	"github.com/davidwhittington/habanero-agent/internal/fleet"
)

type Manager struct {
	fleet  *fleet.Client
	logger *slog.Logger
}

func NewManager(fleetClient *fleet.Client, logger *slog.Logger) *Manager {
	return &Manager{fleet: fleetClient, logger: logger}
}

func (m *Manager) Run(ctx context.Context) {
	m.logger.Info("mesh monitoring started")
	// TODO: discover peers, probe on interval, report results
	<-ctx.Done()
	m.logger.Info("mesh monitoring stopped")
}
