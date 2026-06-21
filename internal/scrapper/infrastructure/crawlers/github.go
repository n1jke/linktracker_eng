package crawlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

var ErrUnauthorized = errors.New("unauthorized github api")

type githubUser struct {
	Login string `json:"login"`
	URL   string `json:"html_url"`
}

type githubPR struct {
	MergedAt string `json:"merged_at"`
}

type githubEvent struct {
	ID        int64      `json:"id"`
	URL       string     `json:"html_url"`
	Title     string     `json:"title"`
	User      githubUser `json:"user"`
	UpdatedAt string     `json:"updated_at"`
	Body      string     `json:"body"`
	PR        *githubPR  `json:"pull_request"`
}

func (w WebCrawler) githubCrawler(ctx context.Context, link domain.TrackedLink) ([]*application.ResourceShot, error) {
	parts := strings.SplitN(link.Path(), "/", 3)
	owner := url.PathEscape(parts[1])
	repo := url.PathEscape(parts[2])

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubTemplate, http.NoBody)
	if err != nil {
		return nil, err
	}

	req.URL.Path = path.Join(req.URL.Path, owner, repo, "issues")
	q := req.URL.Query()
	q.Set("state", "all")
	q.Set("sort", "created")
	q.Set("direction", "desc")
	req.URL.RawQuery = q.Encode()

	req.Header.Set("User-Agent", githubUserAgent)
	req.Header.Set("Authorization", "Bearer "+w.githubToken)

	events := make([]githubEvent, 0)
	if err := getJSON(w.client, req, &events); err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	shots := make([]*application.ResourceShot, 0, len(events))
	for i := range events {
		shot := &application.ResourceShot{
			ID:  events[i].ID,
			URL: events[i].URL,
		}

		updatedAt, err := time.Parse(time.RFC3339, events[i].UpdatedAt)
		if err != nil {
			shot.Description = fmt.Sprintf("fail to parse updated_at: %s", events[i].UpdatedAt)
			shots = append(shots, shot)

			continue
		}

		eventType := "gh|issue"
		if events[i].PR != nil {
			eventType = "gh|pr"
		}

		shots = append(shots, &application.ResourceShot{
			ID:          events[i].ID,
			URL:         events[i].URL,
			EventType:   eventType,
			Description: strings.ToValidUTF8(events[i].Body, ""),
			Author:      events[i].User.URL,
			UpdatedAt:   updatedAt,
		})
	}

	return shots, nil
}
