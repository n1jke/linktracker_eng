package ai

import (
	"log/slog"
	"net/http"

	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/agent/application"
)

var Module = fx.Module(
	"agent",
	fx.Provide(
		fx.Annotate(
			ProvideSummarizer,
			fx.As(new(application.Summarize)),
		),
	),
)

func ProvideSummarizer(logger *slog.Logger, cfg *config.AppConfig, m MetricsRecorder) *Summarizer {
	return NewSummarizer(logger, cfg.Agent.AI.Prompt, cfg.Agent.AI.Model, cfg.Agent.AI.HFToken, &http.Client{
		Timeout: cfg.Agent.AI.HFTimeout,
	}, m)
}
