package config

import (
	"fmt"
	"time"
)

type BotConfig struct {
	TelegramToken    string `env:"TELEGRAM_TOKEN,required,notEmpty"`
	BotURL           string `env:"BOT_URL,required,notEmpty"`
	Scheduler        BotSchedulerConfig
	PushGatewayURL   string        `env:"PUSH_GATEWAY_URL,required,notEmpty"`
	RetryMaxAttempts int           `env:"BOT_RETRY_MAX_ATTEMPTS" envDefault:"3"`
	RetryBaseDelay   time.Duration `env:"BOT_RETRY_BASE_DELAY" envDefault:"200ms"`
	RetryMaxDelay    time.Duration `env:"BOT_RETRY_MAX_DELAY" envDefault:"1s"`
	RetryHTTPCodes   []int         `env:"BOT_RETRY_HTTP_CODES" envSeparator:"," envDefault:"500,502,503,504"`
	RetryGRPCCodes   []uint32      `env:"BOT_RETRY_GRPC_CODES" envSeparator:"," envDefault:"13,14,15"`
}

type BotSchedulerConfig struct {
	InboxBatchSize int           `env:"BOT_INBOX_BATCH_SIZE" envDefault:"25"`
	InboxRelay     time.Duration `env:"BOT_INBOX_RELAY" envDefault:"500ms"`
	InboxClean     time.Duration `env:"BOT_INBOX_CLEAN" envDefault:"1h"`
	MetricsPush    time.Duration `env:"BOT_METRICS_PUSH_INTERVAL" envDefault:"10s"`
}

func (c *BotConfig) Validate() error {
	if c.Scheduler.InboxBatchSize < 1 || c.Scheduler.InboxBatchSize > 15 {
		return fmt.Errorf("inbox batch size: %d", c.Scheduler.InboxBatchSize)
	}

	if c.Scheduler.InboxRelay < 100*time.Millisecond {
		return fmt.Errorf("inbox relay interval small(min: 100ms): %s", c.Scheduler.InboxRelay)
	}

	if c.Scheduler.InboxClean < time.Hour {
		return fmt.Errorf("inbox clean interval small(min: 1h): %s", c.Scheduler.InboxClean)
	}

	if c.Scheduler.MetricsPush < 5*time.Second {
		return fmt.Errorf("metrics push interval small(min: 5s): %s", c.Scheduler.MetricsPush)
	}

	if c.RetryMaxAttempts < 1 {
		return fmt.Errorf("max attempts: %d", c.RetryBaseDelay)
	}

	if c.RetryBaseDelay <= 0 {
		return fmt.Errorf("base delay: %v", c.RetryBaseDelay)
	}

	if c.RetryMaxDelay <= 0 {
		return fmt.Errorf("max delay: %v", c.RetryMaxDelay)
	}

	if c.RetryBaseDelay > c.RetryMaxDelay {
		return fmt.Errorf("base delay > max delay")
	}

	return nil
}

// todo: refactor config for monorepo
type ScrapperConfig struct {
	GithubToken string `env:"GITHUB_TOKEN,required,notEmpty"`
	ScrapperURL string `env:"SCRAPPER_URL,required,notEmpty"`
	BatchSize   int    `env:"SCRAPPER_BATCH_SIZE,required,notEmpty"`
	WorkerCount int    `env:"SCRAPPER_WORKER_COUNT,required,notEmpty"`

	RetryMaxAttempts int           `env:"SCRAPPER_RETRY_MAX_ATTEMPTS" envDefault:"3"`
	RetryBaseDelay   time.Duration `env:"SCRAPPER_RETRY_BASE_DELAY" envDefault:"200ms"`
	RetryMaxDelay    time.Duration `env:"SCRAPPER_RETRY_MAX_DELAY" envDefault:"1s"`
	RetryHTTPCodes   []int         `env:"SCRAPPER_RETRY_HTTP_CODES" envSeparator:"," envDefault:"500,502,503,504"`
	RetryGRPCCodes   []uint32      `env:"SCRAPPER_RETRY_GRPC_CODES" envSeparator:"," envDefault:"13,14,15"`

	RPS   int `env:"SCRAPPER_LIMIT_RPS" envDefault:"1000"`
	Burst int `env:"SCRAPPER_LIMIT_BURST" envDefault:"1500"`

	PushGatewayURL string        `env:"PUSH_GATEWAY_URL,required,notEmpty"`
	MetricsPush    time.Duration `env:"SCRAPPER_METRICS_PUSH_INTERVAL" envDefault:"10s"`
}

func (c *ScrapperConfig) Validate() error {
	if c.BatchSize < 5 || c.BatchSize > 100 {
		return fmt.Errorf("batch size: %d", c.BatchSize)
	}

	if c.WorkerCount < 1 {
		return fmt.Errorf("worker count: %d", c.WorkerCount)
	}

	if c.RetryMaxAttempts < 1 {
		return fmt.Errorf("max attempts: %d", c.RetryBaseDelay)
	}

	if c.RetryBaseDelay <= 0 {
		return fmt.Errorf("base delay: %v", c.RetryBaseDelay)
	}

	if c.RetryMaxDelay <= 0 {
		return fmt.Errorf("max delay: %v", c.RetryMaxDelay)
	}

	if c.RetryBaseDelay > c.RetryMaxDelay {
		return fmt.Errorf("base delay > max delay")
	}

	if c.RPS < 0 {
		return fmt.Errorf("rps for limiter: %v", c.RPS)
	}

	if c.Burst < 0 {
		return fmt.Errorf("burst for limiter: %v", c.Burst)
	}

	if c.RPS > c.Burst {
		return fmt.Errorf("rps > burst for limiter")
	}

	if c.MetricsPush < 5*time.Second {
		return fmt.Errorf("metrics push interval small(min: 5s): %s", c.MetricsPush)
	}

	return nil
}

type SchedulerConfig struct {
	CrawlInterval time.Duration `env:"SCHEDULER_CRAWL_INTERVAL" envDefault:"30s"`
	RelayInterval time.Duration `env:"SCHEDULER_RELAY_INTERVAL" envDefault:"500ms"`
	CleanInterval time.Duration `env:"SCHEDULER_CLEAN_INTERVAL" envDefault:"12h"`
}

func (c *SchedulerConfig) Validate() error {
	if c.CrawlInterval < 10*time.Second {
		return fmt.Errorf("crawl interval small(min: 10s): %s", c.CrawlInterval)
	}

	if c.RelayInterval < 100*time.Millisecond {
		return fmt.Errorf("relay interval small(min: 100ms): %s", c.RelayInterval)
	}

	if c.RelayInterval > c.CrawlInterval {
		return fmt.Errorf("relay interval > crawl interval")
	}

	if c.CleanInterval < time.Hour {
		return fmt.Errorf("clean interval small(min: 1h): %s", c.CleanInterval)
	}

	return nil
}

type AgentConfig struct {
	Filtering   AgentFilteringConfig
	Scheduler   AgentSchedulerConfig
	AI          AgentAIConfig
	WorkerCount int `env:"AGENT_WORKER_COUNT,required,notEmpty"`
}

type AgentFilteringConfig struct {
	StopWords       []string `env:"AGENT_STOP_WORDS" envSeparator:","`
	ExcludedAuthors []string `env:"AGENT_EXCLUDED_AUTHORS" envSeparator:","`
	LowPriority     []string `env:"AGENT_LOW_PRIORITY_WORDS" envSeparator:","`
	HighPriority    []string `env:"AGENT_HIGH_PRIORITY_WORDS" envSeparator:","`
	MinLength       int      `env:"AGENT_MIN_LENGTH" envDefault:"20"`
	Threshold       int      `env:"AGENT_SUMMARIZATION_THRESHOLD" envDefault:"500"`
}

type AgentSchedulerConfig struct {
	GroupWindow     time.Duration `env:"AGENT_GROUP_WINDOW,required,notEmpty"`
	OutboxBatchSize int           `env:"AGENT_OUTBOX_BATCH,required,notEmpty"`
	OutboxRelay     time.Duration `env:"AGENT_OUTBOX_RELAY,required,notEmpty"`
	InboxRelay      time.Duration `env:"AGENT_INBOX_RELAY,required,notEmpty"`
	OutboxClean     time.Duration `env:"AGENT_OUTBOX_CLEAN,required,notEmpty"`
	InboxClean      time.Duration `env:"AGENT_INBOX_CLEAN,required,notEmpty"`
	MetricsPush     time.Duration `env:"AGENT_METRICS_PUSH_INTERVAL" envDefault:"10s"`
}

type AgentAIConfig struct {
	Prompt    string        `env:"AGENT_PROMPT,required,notEmpty"`
	Model     string        `env:"AGENT_MODEL,required,notEmpty"`
	HFToken   string        `env:"AGENT_HG_TOKEN,required,notEmpty"`
	HFTimeout time.Duration `env:"AGENT_HF_TIMEOUT" envDefault:"2s"`
}

func (c *AgentConfig) Validate() error {
	if c.Scheduler.GroupWindow > time.Minute {
		return fmt.Errorf("group window to big(max: 1min): %s", c.Scheduler.GroupWindow)
	}

	if c.Scheduler.OutboxBatchSize < 5 || c.Scheduler.OutboxBatchSize > 100 {
		return fmt.Errorf("outbox batch size: %d", c.Scheduler.OutboxBatchSize)
	}

	if c.Scheduler.OutboxRelay < 100*time.Millisecond {
		return fmt.Errorf("outbox relay interval small(min: 100ms): %s", c.Scheduler.OutboxRelay)
	}

	if c.Scheduler.InboxRelay > c.Scheduler.GroupWindow || c.Scheduler.InboxRelay < c.Scheduler.GroupWindow/4 {
		return fmt.Errorf("inbox relay range(must be < groupwindow  & > groupwindow/4): %s", c.Scheduler.InboxRelay)
	}

	if c.Scheduler.OutboxClean < time.Hour {
		return fmt.Errorf("outbox clean interval small(min: 1h): %s", c.Scheduler.OutboxClean)
	}

	if c.Scheduler.InboxClean < time.Hour {
		return fmt.Errorf("inbox clean interval small(min: 1h): %s", c.Scheduler.InboxClean)
	}

	if c.Scheduler.MetricsPush < 5*time.Second {
		return fmt.Errorf("metrics push interval small(min: 5s): %s", c.Scheduler.MetricsPush)
	}

	if c.Filtering.Threshold < c.Filtering.MinLength {
		return fmt.Errorf("summarization threshold < min length")
	}

	if c.AI.HFTimeout <= 0 {
		return fmt.Errorf("hf timeout must be > 0: %s", c.AI.HFTimeout)
	}

	if c.WorkerCount < 1 {
		return fmt.Errorf("worker count: %d", c.WorkerCount)
	}

	return nil
}
