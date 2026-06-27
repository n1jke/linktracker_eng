package crawlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

//go:generate mockgen -source orchestrator.go -destination=mocks/orchestrator.go -package=mocks
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MetricsRecorder interface {
	Observe(scope, scopeType string, start time.Time)
}

const scope = "crawler"

var ErrInvalidLink = errors.New("unknown link")

type WebCrawler struct {
	client      HTTPClient
	githubToken string
	metrics     MetricsRecorder
}

func NewWebCrawler(githubToken string, m MetricsRecorder) *WebCrawler {
	return &WebCrawler{
		client: &http.Client{
			Timeout: 1 * time.Second,
		},
		githubToken: githubToken,
		metrics:     m,
	}
}

func NewWebCrawlerWithClient(githubToken string, client HTTPClient, m MetricsRecorder) *WebCrawler {
	return &WebCrawler{
		client:      client,
		githubToken: githubToken,
		metrics:     m,
	}
}

func (w *WebCrawler) SearchResource(ctx context.Context, link domain.TrackedLink) ([]*application.ResourceShot, error) {
	defer w.metrics.Observe(scope, string(link.Type()), time.Now())

	switch link.Type() {
	case domain.GitHub:
		return w.githubCrawler(ctx, link)
	case domain.StackOverflow:
		return w.stackOverflowCrawler(ctx, link)
	default:
		return nil, ErrInvalidLink
	}
}
