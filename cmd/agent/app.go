package main

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/agent/application"
	"github.com/n1jke/linktracker_eng/internal/agent/domain"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/ai"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/kafka/consumer"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/kafka/producer"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/repository"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/scheduler"
	"github.com/n1jke/linktracker_eng/internal/agent/infrastructure/telemetry"
	sharedrepo "github.com/n1jke/linktracker_eng/internal/infrastructure/repository"
)

func NewApp() fx.Option {
	return fx.Options(
		fx.Provide(
			ProvideLogger,
		),
		domain.Module,
		application.Module,
		config.Module,
		ai.Module,
		consumer.Module,
		producer.Module,
		repository.Module,
		scheduler.Module,
		sharedrepo.Module,
		telemetry.Module,
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log}
		}),
	)
}

func ProvideLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}
