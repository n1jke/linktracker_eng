package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/n1jke/linktracker_eng/internal/agent/application"
	"github.com/n1jke/linktracker_eng/pkg"
)

func (k *KafkaConsumer) handleFailure(ctx context.Context, msg *kafka.Message, causeErr error, op string) error {
	k.logger.Warn("fail msg processing", slog.String("op", op), slog.Any("err", causeErr), slog.Int("partition", msg.Partition),
		slog.Int64("offset", msg.Offset))

	if err := k.sendDLQ(ctx, msg, causeErr); err != nil {
		return fmt.Errorf("send msg to dlq: %w", err)
	}

	return nil
}

func mapAvroMsg(native any) (*application.RawUpdate, error) {
	data, ok := native.(map[string]any)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid avro msg format"}
	}

	id, ok := data["id"].(int64)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid id in avro msg"}
	}

	url, ok := data["url"].(string)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid url in avro msg"}
	}

	description, ok := data["description"].(string)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid description in avro msg"}
	}

	author, ok := data["author"].(string)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid author in avro msg"}
	}

	eventType, ok := data["event_type"].(string)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid event type in avro msg"}
	}

	updStr, ok := data["updated_at"].(string)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid updatedAt in avro msg"}
	}

	updatedAt, err := time.Parse(time.RFC3339, updStr)
	if err != nil {
		return nil, &pkg.ErrAvroMsg{Op: "invalid time format updatedAt in avro msg"}
	}

	tgChatIDs, ok := data["chat_ids"].([]any)
	if !ok {
		return nil, &pkg.ErrAvroMsg{Op: "invalid chat_ids in avro msg"}
	}

	chatIDs := make([]int64, 0, len(tgChatIDs))

	for _, id := range tgChatIDs {
		if intID, ok := id.(int64); ok {
			chatIDs = append(chatIDs, intID)
		} else {
			return nil, &pkg.ErrAvroMsg{Op: "invalid chatID in avro msg"}
		}
	}

	return &application.RawUpdate{
		ID:          id,
		Link:        url,
		Description: description,
		EventType:   eventType,
		Author:      author,
		ChatIDs:     chatIDs,
		UpdatedAt:   updatedAt,
	}, nil
}
