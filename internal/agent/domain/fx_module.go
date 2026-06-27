package domain

import (
	"go.uber.org/fx"

	"github.com/n1jke/linktracker_eng/config"
)

var Module = fx.Module(
	"domain",
	fx.Provide(
		fx.Private,
		NewFilterConfig,
	),
	fx.Provide(
		NewFilteringPolicy,
	),
)

func NewFilterConfig(cfg *config.AppConfig) *FilterConfig {
	return &FilterConfig{
		StopWords:       cfg.Agent.Filtering.StopWords,
		ExcludedAuthors: toDomainAuthors(cfg.Agent.Filtering.ExcludedAuthors),
		LowPriority:     cfg.Agent.Filtering.LowPriority,
		HighPriority:    cfg.Agent.Filtering.HighPriority,
		MinLength:       cfg.Agent.Filtering.MinLength,
		Threshold:       cfg.Agent.Filtering.Threshold,
	}
}

func toDomainAuthors(src []string) []Author {
	dst := make([]Author, len(src))
	for i := range src {
		dst[i] = Author(src[i])
	}

	return dst
}
