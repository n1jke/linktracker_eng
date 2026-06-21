package consumer

import (
	"errors"

	"github.com/n1jke/linktracker/pkg"
)

type Update struct {
	ID          int64
	URL         string
	Description string
	ChatIDs     []int64
}

func validateUpdate(u *Update) error {
	if u == nil {
		return errors.New("nil update")
	}

	if len(u.ChatIDs) == 0 {
		return errors.New("empty chat_ids")
	}

	return nil
}

func mapAvroMsg(native any) (*Update, error) {
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

	return &Update{
		ID:          id,
		URL:         url,
		Description: description,
		ChatIDs:     chatIDs,
	}, nil
}
