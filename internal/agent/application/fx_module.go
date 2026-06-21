package application

import (
	"log/slog"

	"go.uber.org/fx"

	"github.com/n1jke/linktracker/config"
	"github.com/n1jke/linktracker/internal/agent/domain"
)

var Module = fx.Module(
	"application",
	fx.Provide(
		ProvideAgentService,
	),
)

func ProvideAgentService(logger *slog.Logger, outbox OutboxRepository, policy *domain.FilteringPolicy,
	summarizer Summarize, cfg *config.AppConfig,
) *AgentService {
	return NewAgentService(logger, outbox, policy, summarizer, cfg.Agent.WorkerCount)
}
