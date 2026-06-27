package application

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/n1jke/linktracker_eng/internal/agent/domain"
)

type RawUpdate struct {
	ID          int64
	Link        string
	EventType   string
	Description string
	Author      string
	ChatIDs     []int64
	UpdatedAt   time.Time
}

func (u *RawUpdate) Validate() error {
	if u == nil {
		return errors.New("nil update")
	}

	if len(u.ChatIDs) == 0 {
		return errors.New("empty chat_ids")
	}

	return nil
}

func (u *RawUpdate) ToEvent() *domain.LinkEvent {
	return domain.NewLinkEvent(domain.ID(u.ID), domain.Link(u.Link), domain.Description(u.Description), domain.Author(u.Author))
}

type ProcessedUpdate struct {
	ID          int64
	Link        string
	Description string
	ChatIDs     []int64
	Priority    domain.Priority
}

func UpdateFromEvent(e *domain.LinkEvent, evType string, updatedAt time.Time, chatIDs []int64, p domain.Priority) *ProcessedUpdate {
	return &ProcessedUpdate{
		ID:          int64(e.ID()),
		Link:        string(e.Link()),
		Description: formatDescription(evType, string(e.Author()), updatedAt, string(e.Description()), string(e.Link())),
		ChatIDs:     chatIDs,
		Priority:    p,
	}
}

func GroupUpdates(updates []*ProcessedUpdate, chatID int64) *ProcessedUpdate {
	return &ProcessedUpdate{
		ID:          updates[len(updates)-1].ID,
		Link:        "",
		Description: formatGroupDescription(updates),
		ChatIDs:     []int64{chatID},
		Priority:    MaxPriority(updates),
	}
}

func formatGroupDescription(updates []*ProcessedUpdate) string {
	var b strings.Builder

	b.Grow(len(updates) * 256)

	b.WriteString("You have several updates:\n\n")

	for i, upd := range updates {
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(") ")
		b.WriteString(upd.Description)

		if i != len(updates)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

func MaxPriority(updates []*ProcessedUpdate) domain.Priority {
	p := domain.Low

	for i := range updates {
		if updates[i].Priority.IsHigherThan(p) {
			p = updates[i].Priority
		}
	}

	return p
}

func formatDescription(eventType, author string, timeChanged time.Time, preview, eventURL string) string {
	return fmt.Sprintf(
		"[%s]\nby: %s\nat: %s\n\n%s\n\n%s",
		eventType,
		author,
		timeChanged.Format(time.RFC3339),
		preview,
		eventURL,
	)
}
