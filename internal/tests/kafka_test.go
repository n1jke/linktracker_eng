//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/n1jke/linktracker_eng/internal/bot/infrastructure/kafka/mocks"
	"github.com/riferrei/srclient"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkatc "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/mock/gomock"

	consumer "github.com/n1jke/linktracker_eng/internal/bot/infrastructure/kafka"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	producer "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/kafka"
)

const (
	kafkaImage          = "confluentinc/cp-kafka:8.2.0"
	schemaRegistryImage = "confluentinc/cp-schema-registry:8.2.0"
	consumerGroup       = "test-group"
	topic               = "link-updates"
	dlqTopic            = "link-updates-dlq"
	schema              = `{
		"type": "record",
		"name": "LinkUpdateEvent",
		"namespace": "notification",
		"fields": [
			{"name": "id", "type": "long"},
			{"name": "url", "type": "string"},
			{"name": "description", "type": "string"},
			{"name": "chat_ids", "type": {"type": "array", "items": "long"}},
			{"name": "updated_at", "type": "string"}
		]
	}`
)

func getHeaderValue(headers []kafka.Header, name string) string {
	for _, h := range headers {
		if h.Key == name {
			return string(h.Value)
		}
	}

	return ""
}

func Test_Kafka(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	tests := []struct {
		name    string
		update  *application.ResourceShot
		prepare func(inbox *mocks.MockInboxRepository)
		wantErr bool
		wantDLQ bool
	}{
		{
			name: "single chat success",
			update: &application.ResourceShot{
				ID:          1,
				URL:         "https://tbank.ru",
				Description: "1 success update",
				ChatIDs:     []int64{1},
				UpdatedAt:   time.Now().UTC(),
			},
			prepare: func(inbox *mocks.MockInboxRepository) {
				inbox.EXPECT().AddRecord(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
			wantDLQ: false,
		},
		{
			name: "multiple chats success",
			update: &application.ResourceShot{
				ID:          2,
				URL:         "https://google.com",
				Description: "2 success updates",
				ChatIDs:     []int64{2, 3},
				UpdatedAt:   time.Now().UTC(),
			},
			prepare: func(inbox *mocks.MockInboxRepository) {
				inbox.EXPECT().AddRecord(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			wantErr: false,
			wantDLQ: false,
		},
		{
			name: "invalid update -> dlq",
			update: &application.ResourceShot{
				ID:          3,
				URL:         "https://tested.com",
				Description: "",
				ChatIDs:     []int64{},
				UpdatedAt:   time.Now().UTC(),
			},
			prepare: func(inbox *mocks.MockInboxRepository) {
				inbox.EXPECT().AddRecord(gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr: false,
			wantDLQ: true,
		},
	}

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, inbox, brokers := setupKafkaInfrastructure(ctx, t, ctrl)

	t.Cleanup(func() {
		require.NoError(t, p.Close())
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare(inbox)

			err := p.SendUpdate(ctx, tt.update)
			require.NoError(t, err)

			if tt.wantDLQ {
				dlqReader := kafka.NewReader(kafka.ReaderConfig{
					Brokers:        brokers,
					Topic:          dlqTopic,
					GroupID:        consumerGroup + "-dlq",
					MinBytes:       1,
					MaxBytes:       10e6,
					StartOffset:    kafka.FirstOffset,
					CommitInterval: 0,
				})
				defer func() {
					_ = dlqReader.Close()
				}()

				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				msg, err := dlqReader.ReadMessage(ctx)
				require.NoError(t, err)
				require.Equal(t, dlqTopic, msg.Topic)
				require.NotEmpty(t, getHeaderValue(msg.Headers, "error_msg"))

				return
			}
		})
	}
}

func setupKafkaInfrastructure(ctx context.Context, t *testing.T, ctrl *gomock.Controller) (*producer.KafkaProducer, *mocks.MockInboxRepository, []string) {
	t.Helper()

	net, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := net.Remove(ctx)
		require.NoError(t, err)
	})

	// kafka
	kafkaC, err := kafkatc.Run(
		ctx,
		kafkaImage,
		kafkatc.WithClusterID("test-cluster"),
		network.WithNetworkName([]string{"kafka-broker"}, net.Name),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := kafkaC.Terminate(ctx)
		require.NoError(t, err)
	})

	brokers, err := kafkaC.Brokers(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, brokers)

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)
	t.Cleanup(func() {
		err := conn.Close()
		require.NoError(t, err)
	})

	err = conn.CreateTopics(
		kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1},
		kafka.TopicConfig{Topic: dlqTopic, NumPartitions: 1, ReplicationFactor: 1},
	)
	require.NoError(t, err)

	// schema registry
	srC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: schemaRegistryImage,
			Env: map[string]string{
				"SCHEMA_REGISTRY_HOST_NAME":                    "schema-registry",
				"SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS": "kafka-broker:9092",
				"SCHEMA_REGISTRY_LISTENERS":                    "http://0.0.0.0:8081",
			},
			ExposedPorts:   []string{"8081/tcp"},
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"schema-registry"}},
			WaitingFor:     wait.ForHTTP("/subjects").WithPort("8081/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		err := srC.Terminate(ctx)
		require.NoError(t, err)
	})

	srHost, _ := srC.Host(ctx)
	srPort, _ := srC.MappedPort(ctx, "8081")
	srExternalURL := fmt.Sprintf("http://%s:%s", srHost, srPort.Port())

	srClient := srclient.CreateSchemaRegistryClient(srExternalURL)
	_, err = srClient.CreateSchema(topic+"-value", schema, srclient.Avro)
	require.NoError(t, err)

	producerConfig := &producer.KafkaConfig{
		Attempts:          3,
		BatchSize:         10,
		Topic:             topic,
		Brokers:           brokers,
		SchemaRegistryURL: srExternalURL,
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	p, err := producer.NewKafkaProducer(logger, producerConfig)
	require.NoError(t, err)

	inbox := mocks.NewMockInboxRepository(ctrl)
	c, err := consumer.NewKafkaConsumer(logger, &consumer.KafkaConfig{
		Topic:             topic,
		DLQTopic:          dlqTopic,
		Brokers:           brokers,
		SchemaRegistryURL: srExternalURL,
		ConsumerGroup:     consumerGroup,
		Attempts:          3,
	}, inbox)
	require.NoError(t, err)

	consumerCtx, cancel := context.WithCancel(ctx)

	done := make(chan error, 1)
	go func() {
		done <- c.Start(consumerCtx)
	}()

	// todo
	t.Cleanup(func() {
		cancel()

		_ = c.Stop(context.Background())

		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	})

	return p, inbox, brokers
}
