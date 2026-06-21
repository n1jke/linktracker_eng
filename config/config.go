package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
)

type AppConfig struct {
	Bot            BotConfig
	Scrapper       ScrapperConfig
	Agent          AgentConfig
	Scheduler      SchedulerConfig
	Communication  CommunicationConfig
	Timeouts       TimeoutsConfig
	DB             DatabaseConfig
	Valkey         ValkeyConfig
	Kafka          KafkaConfig
	CircuitBreaker BreakerConfig
}

func (c *AppConfig) Validate() error {
	if err := c.Bot.Validate(); err != nil {
		return fmt.Errorf("bot config: %w", err)
	}

	if err := c.Scrapper.Validate(); err != nil {
		return fmt.Errorf("scrapper config: %w", err)
	}

	if err := c.Agent.Validate(); err != nil {
		return fmt.Errorf("agent config: %w", err)
	}

	if err := c.Communication.Validate(); err != nil {
		return fmt.Errorf("communication config: %w", err)
	}

	if err := c.Timeouts.Validate(c.Communication.RequestProtocol); err != nil {
		return err
	}

	if err := c.DB.Validate(); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	if err := c.Scheduler.Validate(); err != nil {
		return fmt.Errorf("scheduler config: %w", err)
	}

	if c.Communication.NotificationProtocol == ProtocolKafka {
		if err := c.Kafka.Validate(); err != nil {
			return fmt.Errorf("kafka config: %w", err)
		}
	}

	if c.Valkey.Enabled {
		if err := c.Valkey.Validate(); err != nil {
			return fmt.Errorf("valkey config: %w", err)
		}
	}

	if err := c.CircuitBreaker.Validate(); err != nil {
		return fmt.Errorf("circuit breaker config: %w", err)
	}

	return nil
}

func LoadEnv() error {
	return nil
}

func LoadConfig() (*AppConfig, error) {
	cfg := &AppConfig{}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
