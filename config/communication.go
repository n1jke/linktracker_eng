package config

import "fmt"

type ProtocolType string

const (
	ProtocolHTTP  ProtocolType = "http"
	ProtocolGRPC  ProtocolType = "grpc"
	ProtocolKafka ProtocolType = "kafka"
)

type CommunicationConfig struct {
	RequestProtocol      ProtocolType `env:"REQUEST_PROTOCOL" envDefault:"grpc"`
	NotificationProtocol ProtocolType `env:"UPDATES_PROTOCOL" envDefault:"kafka"`
}

func (c *CommunicationConfig) Validate() error {
	if c.RequestProtocol != ProtocolHTTP && c.RequestProtocol != ProtocolGRPC {
		return fmt.Errorf("api protocol type: %q", c.RequestProtocol)
	}

	if c.NotificationProtocol != ProtocolHTTP && c.NotificationProtocol != ProtocolGRPC &&
		c.NotificationProtocol != ProtocolKafka {
		return fmt.Errorf("updates protocol type: %q", c.NotificationProtocol)
	}

	return nil
}

type KafkaConfig struct {
	BootstrapServers  []string `env:"KAFKA_BOOTSTRAP_SERVERS,required,notEmpty" envSeparator:","`
	Username          string   `env:"KAFKA_USER,required,notEmpty" json:"-" yaml:"-"`
	Password          string   `env:"KAFKA_PASSWORD,required,notEmpty" json:"-" yaml:"-"`
	RawUpdatesTopic   string   `env:"KAFKA_RAW_UPDATES_TOPIC" envDefault:"link-raw-updates"`
	RawDLQTopic       string   `env:"KAFKA_RAW_DLQ_TOPIC" envDefault:"link-raw-updates-dlq"`
	UpdatesTopic      string   `env:"KAFKA_PREP_UPDATES_TOPIC" envDefault:"link-processed-updates"`
	DLQTopic          string   `env:"KAFKA_PREP_DLQ_TOPIC" envDefault:"link-processed-updates-dlq"`
	RawConsumerGroup  string   `env:"KAFKA_RAW_CONSUMER_GROUP" envDefault:"agent"`
	PrepConsumerGroup string   `env:"KAFKA_PREP_CONSUMER_GROUP" envDefault:"bot"`
	ConsumerAttempts  int      `env:"KAFKA_CONSUMER_ATTEMPTS" envDefault:"3"`
	ProducerAttempts  int      `env:"KAFKA_PRODUCER_ATTEMPTS" envDefault:"3"`
	ProducerBatchSize int      `env:"KAFKA_PRODUCER_BATCH_SIZE" envDefault:"10"`
	SchemaRegistryURL string   `env:"SCHEMA_REGISTRY_URL"`
}

func (c *KafkaConfig) Validate() error {
	if c.ConsumerAttempts < 1 {
		return fmt.Errorf("consumer attempts: %d", c.ConsumerAttempts)
	}

	if c.ProducerAttempts < 1 {
		return fmt.Errorf("producer attempts: %d", c.ProducerAttempts)
	}

	if c.ProducerBatchSize < 1 {
		return fmt.Errorf("producer batch size: %d", c.ProducerBatchSize)
	}

	if c.SchemaRegistryURL == "" {
		return fmt.Errorf("empty schema registry url")
	}

	return nil
}
