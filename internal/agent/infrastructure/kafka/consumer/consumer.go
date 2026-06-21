package consumer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/riferrei/srclient"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"

	"github.com/n1jke/linktracker/internal/agent/application"
)

const (
	magicByte       byte = 0
	schemaValueSize int  = 5
)

var ErrDuplicateMsg = errors.New("duplicate message")

type InboxRepository interface {
	AddRecord(ctx context.Context, raw *application.RawUpdate) error
}

type KafkaConfig struct {
	Topic             string
	DLQTopic          string
	ConsumerGroup     string
	Brokers           []string
	username          string
	password          string
	SchemaRegistryURL string
	RetryAttempts     int
}

type KafkaConsumer struct {
	logger *slog.Logger
	reader *kafka.Reader
	dlq    *kafka.Writer
	inbox  InboxRepository
	client *srclient.SchemaRegistryClient
}

func NewKafkaConsumer(logger *slog.Logger, cfg *KafkaConfig, inbox InboxRepository) (*KafkaConsumer, error) {
	logger = logger.With("module", "kafka-consumer")

	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	transport := &kafka.Transport{}

	if cfg.password != "" && cfg.username != "" {
		auth := plain.Mechanism{
			Username: cfg.username,
			Password: cfg.password,
		}
		dialer.SASLMechanism = auth
		transport.SASL = auth
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.ConsumerGroup,
		CommitInterval: 0,
		Dialer:         dialer,
		IsolationLevel: kafka.ReadCommitted,
	})

	dlq := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Topic:                  cfg.DLQTopic,
		MaxAttempts:            cfg.RetryAttempts,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false,
		Transport:              transport,
	}

	client := srclient.NewSchemaRegistryClient(cfg.SchemaRegistryURL)
	client.CachingEnabled(true)

	return &KafkaConsumer{
		logger: logger,
		reader: reader,
		dlq:    dlq,
		inbox:  inbox,
		client: client,
	}, nil
}

func (k *KafkaConsumer) Consume(ctx context.Context) error {
	for {
		msg, err := k.reader.FetchMessage(ctx)
		if err != nil {
			k.logger.Error("read msg from kafka", slog.Any("err", err))
			return fmt.Errorf("read msg from kafka %w", err)
		}

		raw, err := k.decodeMessage(&msg)
		if err != nil {
			return k.handleFailure(ctx, &msg, err, "decode avro")
		}

		if err := raw.Validate(); err != nil {
			return k.handleFailure(ctx, &msg, err, "validate payload")
		}

		if err := k.inbox.AddRecord(ctx, raw); err != nil {
			if !errors.Is(err, ErrDuplicateMsg) {
				return k.handleFailure(ctx, &msg, err, "add to inbox")
			}
		}

		if err = k.reader.CommitMessages(ctx, msg); err != nil {
			k.logger.Error("commit msg", slog.Any("err", err))
			return fmt.Errorf("commit msg: %w", err)
		}
	}
}

func (k *KafkaConsumer) Start(ctx context.Context) error {
	return k.Consume(ctx)
}

func (k *KafkaConsumer) Stop() error {
	var stopErr error
	if err := k.reader.Close(); err != nil {
		stopErr = err
	}

	if err := k.dlq.Close(); err != nil {
		if stopErr != nil {
			return errors.Join(stopErr, err)
		}

		return err
	}

	return stopErr
}

func (k *KafkaConsumer) decodeMessage(msg *kafka.Message) (*application.RawUpdate, error) {
	if len(msg.Value) < schemaValueSize {
		return nil, fmt.Errorf("invalid msg.Value payload: %d", len(msg.Value))
	}

	if msg.Value[0] != magicByte {
		return nil, fmt.Errorf("invalid confluent 1-st byte: %d", msg.Value[0])
	}

	schemaID := int(binary.BigEndian.Uint32(msg.Value[1:schemaValueSize]))

	schema, err := k.client.GetSchema(schemaID)
	if err != nil {
		return nil, fmt.Errorf("get schema by id %d: %w", schemaID, err)
	}

	native, _, err := schema.Codec().NativeFromBinary(msg.Value[schemaValueSize:])
	if err != nil {
		return nil, err
	}

	return mapAvroMsg(native)
}

func (k *KafkaConsumer) sendDLQ(ctx context.Context, msg *kafka.Message, err error) error {
	dlqMsg := kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: append(msg.Headers,
			kafka.Header{Key: "source_topic", Value: []byte(msg.Topic)},
			kafka.Header{Key: "source_partition", Value: []byte(strconv.Itoa(msg.Partition))},
			kafka.Header{Key: "source_offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
			kafka.Header{Key: "error_msg", Value: []byte(err.Error())},
		),
	}

	if err := k.dlq.WriteMessages(ctx, dlqMsg); err != nil {
		k.logger.Error("write dlq msg", slog.Any("err", err))
		return fmt.Errorf("write dlq msg: %w", err)
	}

	return nil
}
