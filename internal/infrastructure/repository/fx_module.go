package repository

import (
	"go.uber.org/fx"

	agentscheduler "github.com/n1jke/linktracker_eng/internal/agent/infrastructure/scheduler"
	botscheduler "github.com/n1jke/linktracker_eng/internal/bot/infrastructure/scheduler"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/scheduler"
)

var Module = fx.Module(
	"shared-repository",
	fx.Provide(
		fx.Annotate(
			NewTxChain,
			fx.As(new(botscheduler.Transactor)),
			fx.As(new(agentscheduler.Transactor)),
			fx.As(new(scheduler.Transactor)),
			fx.As(new(application.Transactor)),
		),
	),
)
