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
)

const (
	magicByte       byte = 0
	schemaValueSize int  = 5
)

//go:generate mockgen -source consumer.go -destination=mocks/mocks.go -package=mocks
type InboxRepository interface {
	AddRecord(ctx context.Context, rec *InboxRecord) error
}

var ErrDuplicateMsg = errors.New("duplicate message")

type InboxRecord struct {
	Record         *Update
	IdempotencyKey string
}

type KafkaConfig struct {
	Attempts          int
	Topic             string
	DLQTopic          string
	ConsumerGroup     string
	Brokers           []string
	username          string
	password          string
	SchemaRegistryURL string
}

func NewKafkaConfig(attempts int, topic, dlqTopic, consumerGroup string, brokers []string,
	username, password, schemaRegistry string,
) *KafkaConfig {
	return &KafkaConfig{
		Attempts:          attempts,
		Topic:             topic,
		DLQTopic:          dlqTopic,
		ConsumerGroup:     consumerGroup,
		Brokers:           brokers,
		username:          username,
		password:          password,
		SchemaRegistryURL: schemaRegistry,
	}
}

type KafkaConsumer struct {
	logger *slog.Logger
	reader *kafka.Reader
	dlq    *kafka.Writer
	client *srclient.SchemaRegistryClient
	inbox  InboxRepository
}

func NewKafkaConsumer(logger *slog.Logger, config *KafkaConfig, inbox InboxRepository) (*KafkaConsumer, error) {
	logger = logger.With("module", "kafka-consumer")

	dialer := &kafka.Dialer{Timeout: 3 * time.Second}
	transport := &kafka.Transport{}

	if config.username != "" && config.password != "" {
		auth := plain.Mechanism{
			Username: config.username,
			Password: config.password,
		}
		dialer.SASLMechanism = auth
		transport.SASL = auth
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        config.Brokers,
		Topic:          config.Topic,
		GroupID:        config.ConsumerGroup,
		CommitInterval: 0,
		Dialer:         dialer,
		IsolationLevel: kafka.ReadCommitted,
	})

	dlq := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		Topic:                  config.DLQTopic,
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false,
		Transport:              transport,
	}

	client := srclient.NewSchemaRegistryClient(config.SchemaRegistryURL)
	client.CachingEnabled(true)

	return &KafkaConsumer{
		logger: logger,
		reader: reader,
		dlq:    dlq,
		client: client,
		inbox:  inbox,
	}, nil
}

func (k *KafkaConsumer) PostUpdates(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := k.reader.FetchMessage(ctx)
		if err != nil {
			k.logger.Error("read msg from kafka", slog.Any("err", err))
			return fmt.Errorf("read msg from kafka: %w", err)
		}

		err = k.processMessage(ctx, &msg)
		if err != nil {
			return err
		}

		if err = k.reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("commit msg: %w", err)
		}
	}
}

func (k *KafkaConsumer) Start(ctx context.Context) error {
	return k.PostUpdates(ctx)
}

func (k *KafkaConsumer) Stop(_ context.Context) error {
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

func (k *KafkaConsumer) processMessage(ctx context.Context, msg *kafka.Message) error {
	update, err := k.decodeMessage(msg)
	if err != nil {
		k.logger.Error("decode msg", slog.Any("err", err), slog.Int("partition", msg.Partition), slog.Int64("offset", msg.Offset))
		return k.sendDLQ(ctx, msg, err)
	}

	if err = validateUpdate(update); err != nil {
		k.logger.Error("update payload", slog.Any("err", err), slog.Int("partition", msg.Partition), slog.Int64("offset", msg.Offset))
		return k.sendDLQ(ctx, msg, err)
	}

	err = k.inbox.AddRecord(ctx, &InboxRecord{Record: update, IdempotencyKey: findIdempKey(msg.Headers)})
	if err != nil {
		if errors.Is(err, ErrDuplicateMsg) {
			k.logger.Warn("find duplicate msg", slog.Int("partition", msg.Partition), slog.Int64("offset", msg.Offset))
			return nil
		}

		k.logger.Error("add inbox record", slog.Any("err", err), slog.Int("partition", msg.Partition), slog.Int64("offset", msg.Offset))

		return k.sendDLQ(ctx, msg, err)
	}

	return nil
}

func (k *KafkaConsumer) decodeMessage(msg *kafka.Message) (*Update, error) {
	if len(msg.Value) < schemaValueSize {
		return nil, fmt.Errorf("invalid msg.Value payload: %b", len(msg.Value))
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

func findIdempKey(headers []kafka.Header) string {
	for i := range headers {
		if headers[i].Key == "idempotency-key" {
			return string(headers[i].Value)
		}
	}

	return ""
}
