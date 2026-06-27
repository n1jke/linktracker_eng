package crawlers_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/crawlers"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/crawlers/mocks"
)

const githubToken = "test-token"

func TestWebCrawler_SearchResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		link    *domain.TrackedLink
		setup   func(client *mocks.MockHTTPClient)
		wantErr bool
	}{
		{
			name: "github 200 resp",
			link: newLink(t, "https://github.com/n1jke/oop-bsuir-2026"),
			setup: func(client *mocks.MockHTTPClient) {
				client.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`[{"id":1,"html_url":"https://github.com/n1jke/oop-bsuir-2026", "title":"Lr 6",
						"user":{"login":"n1jke", "html_url":"https://github.com/n1jke"},"updated_at":"2026-05-05T17:00:00Z", "body":""}]`)),
				}, nil)
			},
		},
		{
			name: "github unauthorized",
			link: newLink(t, "https://github.com/owner/repo"),
			setup: func(client *mocks.MockHTTPClient) {
				client.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(strings.NewReader(``)),
				}, nil)
			},
			wantErr: true,
		},
		{
			name: "so valid",
			link: newLink(t, "https://stackoverflow.com/questions/58664093"),
			setup: func(client *mocks.MockHTTPClient) {
				client.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(`{"items":[{"question_id":58664093,"link":"https://stackoverflow.com/questions/58664093",
						"title":"Some question title","last_activity_date":1}]}`)),
				}, nil)
			},
		},
		{
			name: "so empty",
			link: newLink(t, "https://stackoverflow.com/questions/1"),
			setup: func(client *mocks.MockHTTPClient) {
				client.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
				}, nil)
			},
			wantErr: false,
		},
		{
			name: "so request timeout",
			link: newLink(t, "https://stackoverflow.com/questions/1"),
			setup: func(client *mocks.MockHTTPClient) {
				client.EXPECT().Do(gomock.Any()).Return(&http.Response{
					StatusCode: http.StatusRequestTimeout,
					Body:       io.NopCloser(strings.NewReader(`{"items":[]}`)),
				}, nil)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mocks.NewMockHTTPClient(ctrl)
			metrics := &metricsMock{}

			tt.setup(client)

			w := crawlers.NewWebCrawlerWithClient(githubToken, client, metrics)

			_, err := w.SearchResource(context.Background(), *tt.link)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func newLink(t *testing.T, url string) *domain.TrackedLink {
	t.Helper()

	l, err := domain.NewTrackedLink(url)
	require.NoError(t, err)

	return l
}

type metricsMock struct{}

func (m *metricsMock) Observe(_, _ string, _ time.Time) {}
