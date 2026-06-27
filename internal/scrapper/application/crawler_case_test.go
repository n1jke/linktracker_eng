package application_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application/mocks"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

const (
	workerCount = 32
	batchSize   = 128
)

func TestCrawlerService_NotifySubscribers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prepare func(links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository,
			crawler *mocks.MockSourceCrawler, tx *mocks.MockTransactor, outbox *mocks.MockOutboxRepository)
		wantErr bool
	}{
		{
			name: "storage error",
			prepare: func(links *mocks.MockLinksRepository, _ *mocks.MockSubscriptionsRepository, _ *mocks.MockSourceCrawler,
				_ *mocks.MockTransactor, _ *mocks.MockOutboxRepository,
			) {
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return(nil, nil, assert.AnError)
			},
			wantErr: true,
		},
		{
			name: "0 links",
			prepare: func(links *mocks.MockLinksRepository, _ *mocks.MockSubscriptionsRepository, _ *mocks.MockSourceCrawler,
				_ *mocks.MockTransactor, _ *mocks.MockOutboxRepository,
			) {
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return(nil, nil, nil)
			},
		},
		{
			name: "0 updates",
			prepare: func(links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository,
				crawler *mocks.MockSourceCrawler, _ *mocks.MockTransactor, _ *mocks.MockOutboxRepository,
			) {
				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return([]*domain.TrackedLink{linkObj}, nil, nil)

				shot := &application.ResourceShot{UpdatedAt: time.Now().UTC()}
				crawler.EXPECT().SearchResource(gomock.Any(), *linkObj).Return([]*application.ResourceShot{shot}, nil)

				sub := domain.NewLinkSubscription(1, linkObj.ID(), linkObj.Path())
				sub.SetLastUpdate(shot.UpdatedAt)
				subs.EXPECT().GetSubscriptionsByResourceID(gomock.Any(), linkObj.ID()).Return([]*domain.LinkSubscription{sub}, nil)
			},
		},
		{
			name: "notifier error",
			prepare: func(links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository, crawler *mocks.MockSourceCrawler,
				tx *mocks.MockTransactor, outbox *mocks.MockOutboxRepository,
			) {
				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return([]*domain.TrackedLink{linkObj}, nil, nil)

				shot := &application.ResourceShot{UpdatedAt: time.Now().UTC()}
				crawler.EXPECT().SearchResource(gomock.Any(), *linkObj).Return([]*application.ResourceShot{shot}, nil)

				sub := domain.NewLinkSubscription(1, linkObj.ID(), linkObj.Path())
				sub.SetLastUpdate(shot.UpdatedAt.Add(-time.Minute))
				subs.EXPECT().GetSubscriptionsByResourceID(gomock.Any(), linkObj.ID()).Return([]*domain.LinkSubscription{sub}, nil)

				tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txFunc func(context.Context) error) error {
						return txFunc(ctx)
					})
				subs.EXPECT().UpdateSubscriptionLastUpdate(gomock.Any(), int64(1), linkObj.ID(), shot.UpdatedAt).Return(nil)
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
		},
		{
			name: "crawler error with shot sends update",
			prepare: func(links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository, crawler *mocks.MockSourceCrawler,
				tx *mocks.MockTransactor, outbox *mocks.MockOutboxRepository,
			) {
				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return([]*domain.TrackedLink{linkObj}, nil, nil)

				shot := &application.ResourceShot{
					URL:         linkObj.Path(),
					Description: "crawl fail with timeout",
					UpdatedAt:   time.Now().UTC(),
				}
				crawler.EXPECT().SearchResource(gomock.Any(), *linkObj).Return([]*application.ResourceShot{shot}, assert.AnError)

				tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context,
					txFunc func(context.Context) error,
				) error {
					return txFunc(ctx)
				})
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Return(nil)

				sub := domain.NewLinkSubscription(1, linkObj.ID(), linkObj.Path())
				subs.EXPECT().GetSubscriptionsByResourceID(gomock.Any(), linkObj.ID()).Return([]*domain.LinkSubscription{sub}, nil)
			},
		},
		{
			name: "update last update error is non fatal",
			prepare: func(links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository, crawler *mocks.MockSourceCrawler,
				tx *mocks.MockTransactor, _ *mocks.MockOutboxRepository,
			) {
				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinksBatch(gomock.Any(), batchSize, (*time.Time)(nil)).Return([]*domain.TrackedLink{linkObj}, nil, nil)

				shot := &application.ResourceShot{UpdatedAt: time.Now().UTC()}
				crawler.EXPECT().SearchResource(gomock.Any(), *linkObj).Return([]*application.ResourceShot{shot}, nil)

				sub := domain.NewLinkSubscription(1, linkObj.ID(), linkObj.Path())
				sub.SetLastUpdate(shot.UpdatedAt.Add(-time.Minute))
				subs.EXPECT().GetSubscriptionsByResourceID(gomock.Any(), linkObj.ID()).Return([]*domain.LinkSubscription{sub}, nil)

				tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, txFunc func(context.Context) error) error {
						return txFunc(ctx)
					})
				subs.EXPECT().UpdateSubscriptionLastUpdate(gomock.Any(), int64(1), linkObj.ID(), shot.UpdatedAt).Return(assert.AnError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			crawler := mocks.NewMockSourceCrawler(ctrl)
			tx := mocks.NewMockTransactor(ctrl)
			outbox := mocks.NewMockOutboxRepository(ctrl)
			metrics := &metricsMock{}

			tt.prepare(linksRepo, subsRepo, crawler, tx, outbox)

			svc := application.NewCrawlerService(logger, tx, subsRepo, linksRepo, outbox, crawler, metrics, workerCount, batchSize)

			err := svc.NotifySubscribers(context.Background())
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type metricsMock struct{}

func (m *metricsMock) SetLinksOnTrack(int) {}
