package config

import (
	"github.com/sony/gobreaker/v2"

	consumer "github.com/n1jke/linktracker_eng/internal/bot/infrastructure/kafka"
	producer "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/kafka"
	cache "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository/valkey"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/scheduler"
	"github.com/n1jke/linktracker_eng/pkg/retry"
)

func (c *AppConfig) MapSchedulerConfig() *scheduler.Config {
	return &scheduler.Config{
		BatchSize:   c.Scrapper.BatchSize,
		CrawlGap:    c.Scheduler.CrawlInterval,
		KafkaGap:    c.Scheduler.RelayInterval,
		CleanGap:    c.Scheduler.CleanInterval,
		MetricsPush: c.Scrapper.MetricsPush,
	}
}

func (c *AppConfig) MapValkeyConfig() *cache.ValkeyConfig {
	return cache.NewValkeyConfig(c.Valkey.Endpoints, c.Valkey.MasterName, c.Valkey.Username, c.Valkey.Password, c.Valkey.TTL)
}

func (c *AppConfig) MapProducerConfig() *producer.KafkaConfig {
	return producer.NewKafkaConfig(c.Kafka.ProducerAttempts, c.Kafka.ProducerBatchSize, c.Kafka.RawUpdatesTopic,
		c.Kafka.BootstrapServers, c.Kafka.Username, c.Kafka.Password, c.Kafka.SchemaRegistryURL)
}

func (c *AppConfig) MapConsumerConfig() *consumer.KafkaConfig {
	return consumer.NewKafkaConfig(c.Kafka.ConsumerAttempts, c.Kafka.UpdatesTopic, c.Kafka.DLQTopic, c.Kafka.PrepConsumerGroup,
		c.Kafka.BootstrapServers, c.Kafka.Username, c.Kafka.Password, c.Kafka.SchemaRegistryURL)
}

func (c *AppConfig) MapBotRetryConfig() *retry.Config {
	return &retry.Config{
		MaxAttempts: c.Bot.RetryMaxAttempts,
		BaseDelay:   c.Bot.RetryBaseDelay,
		MaxDelay:    c.Bot.RetryMaxDelay,
	}
}

func (c *AppConfig) MapScrapperRetryConfig() *retry.Config {
	return &retry.Config{
		MaxAttempts: c.Scrapper.RetryMaxAttempts,
		BaseDelay:   c.Scrapper.RetryBaseDelay,
		MaxDelay:    c.Scrapper.RetryMaxDelay,
	}
}

func (c *AppConfig) MapBreakerConfig(name string) *gobreaker.Settings {
	threshold := c.CircuitBreaker.FailureRateThreshold

	return &gobreaker.Settings{
		Name:         name,
		MaxRequests:  c.CircuitBreaker.MaxRequests,
		Interval:     c.CircuitBreaker.Interval,
		BucketPeriod: c.CircuitBreaker.BucketPeriod,
		Timeout:      c.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			rate := float64(counts.TotalFailures) / float64(counts.Requests) * 100
			return rate >= threshold
		},
	}
}
