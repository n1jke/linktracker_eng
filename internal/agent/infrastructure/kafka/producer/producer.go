package producer

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riferrei/srclient"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"

	"github.com/n1jke/linktracker/internal/agent/application"
)

const writeTimeout = 3 * time.Second

type KafkaConfig struct {
	Attempts          int
	BatchSize         int
	Topic             string
	Brokers           []string
	username          string
	password          string
	SchemaRegistryURL string
}

type KafkaProducer struct {
	logger *slog.Logger
	schema *srclient.Schema
	writer *kafka.Writer
}

func NewKafkaProducer(logger *slog.Logger, cfg *KafkaConfig) (*KafkaProducer, error) {
	logger = logger.With("module", "kafka-producer")

	transport := &kafka.Transport{}
	if cfg.username != "" && cfg.password != "" {
		transport.SASL = plain.Mechanism{
			Username: cfg.username,
			Password: cfg.password,
		}
	}

	writer := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		MaxAttempts:            cfg.Attempts,
		BatchSize:              cfg.BatchSize,
		BatchTimeout:           2 * time.Second,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false,
		Transport:              transport,
	}

	client := srclient.NewSchemaRegistryClient(cfg.SchemaRegistryURL)

	subject := fmt.Sprintf("%s-value", cfg.Topic)

	schema, err := client.GetLatestSchema(subject)
	if err != nil {
		return nil, fmt.Errorf("get latest schema for subject: %w", err)
	}

	return &KafkaProducer{
		logger: logger,
		schema: schema,
		writer: writer,
	}, nil
}

func (p *KafkaProducer) Publish(ctx context.Context, update *application.ProcessedUpdate) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	native := mapResourceShotToAvro(update)

	data, err := p.schema.Codec().BinaryFromNative(nil, native)
	if err != nil {
		p.logger.Error("avro encoding", slog.Any("err", err))
		return fmt.Errorf("avro encoding: %w", err)
	}

	payload := make([]byte, 5+len(data))
	payload[0] = 0
	binary.BigEndian.PutUint32(payload[1:5], uint32(p.schema.ID())) //nolint:gosec // schema ID from SR is always uint32
	copy(payload[5:], data)

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key: []byte(update.Link),
		Headers: []kafka.Header{
			{Key: "idempotency-key", Value: []byte(uuid.New().String())},
		},
		Value: payload,
	})
	if err != nil {
		p.logger.Error("send update to kafka", slog.Any("err", err))
		return fmt.Errorf("send update to kafka: %w", err)
	}

	return nil
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

func mapResourceShotToAvro(update *application.ProcessedUpdate) map[string]any {
	chatIDs := make([]any, 0, len(update.ChatIDs))
	for i := range update.ChatIDs {
		chatIDs = append(chatIDs, update.ChatIDs[i])
	}

	return map[string]any{
		"id":          update.ID,
		"url":         update.Link,
		"description": update.Description,
		"chat_ids":    chatIDs,
		"priority":    string(update.Priority),
	}
}
