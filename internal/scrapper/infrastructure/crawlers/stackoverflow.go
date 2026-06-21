package crawlers

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type soUser struct {
	DisplayName string `json:"display_name"`
	Link        string `json:"link"`
}

type soAnswer struct {
	AnswerID  int64  `json:"answer_id"`
	UpdatedAt int64  `json:"last_edit_date"`
	Body      string `json:"body"`
	Link      string `json:"link"`
	Owner     soUser `json:"owner"`
}

type soAnswersResponse struct {
	Items []soAnswer `json:"items"`
}

type soEvent struct {
	ID        int64
	Title     string
	Author    string
	AuthorURL string
	URL       string
	Preview   string
	CreatedAt time.Time
}

func (w WebCrawler) stackOverflowCrawler(ctx context.Context, link domain.TrackedLink) ([]*application.ResourceShot, error) {
	parts := strings.SplitN(link.Path(), "/", 3)
	questionID := url.PathEscape(parts[2])

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stackOverflowTemplate, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.URL.Path = path.Join(req.URL.Path, questionID)
	q := req.URL.Query()
	q.Set("site", "stackoverflow")
	q.Set("order", "desc")
	q.Set("sort", "creation")
	req.URL.RawQuery = q.Encode()

	resp := soAnswersResponse{}
	if err := getJSON(w.client, req, &resp); err != nil {
		return nil, err
	}

	events := buildSOEvents(link.Path(), resp.Items)
	if len(events) == 0 {
		return nil, nil
	}

	shots := make([]*application.ResourceShot, 0, len(events))
	for i := range events {
		shots = append(shots, &application.ResourceShot{
			ID:          events[i].ID,
			URL:         events[i].URL,
			EventType:   "so|answer",
			Description: events[i].Preview,
			Author:      events[i].Author,
			UpdatedAt:   events[i].CreatedAt.UTC(),
		})
	}

	return shots, nil
}

func buildSOEvents(title string, answers []soAnswer) []*soEvent {
	events := make([]*soEvent, 0, len(answers))
	for i := range answers {
		events = append(events, &soEvent{
			ID:        answers[i].AnswerID,
			Title:     title,
			Author:    answers[i].Owner.DisplayName,
			AuthorURL: answers[i].Owner.Link,
			URL:       answers[i].Link,
			Preview:   strings.ToValidUTF8(answers[i].Body, ""),
			CreatedAt: time.Unix(answers[i].UpdatedAt, 0),
		})
	}

	return events
}
