//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/valkey"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application/mocks"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
	cache "github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/repository/valkey"
)

func Test_Valkey(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	t.Parallel()

	ctx := context.Background()
	valkeyClient := setupCache(ctx, t)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	tests := []struct {
		name     string
		clientID int64
		link     *domain.LinkSubscription
		prepare  func(clientID int64, v *cache.Valkey, tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository)
		test     func(clientID int64, s *application.CachedScrapperService) error
		check    func(clientID int64, link *domain.LinkSubscription, t *testing.T, v *cache.Valkey)
		wantErr  bool
	}{
		{
			name:     "empty cache & fill it",
			clientID: 1,
			link:     domain.NewLinkSubscription(1, uuid.New(), "https://github.com/golangci/golangci-lint"),
			prepare: func(clientID int64, v *cache.Valkey, tx *mocks.MockTransactor, users *mocks.MockUserRepository, _ *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), clientID).Return(domain.NewClient(clientID), nil)
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), clientID).Return([]*domain.LinkSubscription{domain.NewLinkSubscription(clientID, uuid.New(), "https://github.com/golangci/golangci-lint")}, nil)
			},
			test: func(clientID int64, s *application.CachedScrapperService) error {
				_, err := s.GetLinks(ctx, clientID)
				return err
			},
			check: func(clientID int64, link *domain.LinkSubscription, t *testing.T, v *cache.Valkey) {
				time.Sleep(50 * time.Millisecond)

				cacheKey := fmt.Sprintf("links:%d", clientID)
				data, ok, err := v.Get(ctx, cacheKey)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, "https://github.com/golangci/golangci-lint", data[0].Link())
			},
		},
		{
			name:     "hit cache",
			clientID: 2,
			link:     domain.NewLinkSubscription(2, uuid.New(), "https://github.com/valkey-io/valkey"),
			prepare: func(clientID int64, v *cache.Valkey, _ *mocks.MockTransactor, _ *mocks.MockUserRepository, _ *mocks.MockLinksRepository, _ *mocks.MockSubscriptionsRepository) {
				links := []*domain.LinkSubscription{domain.NewLinkSubscription(clientID, uuid.New(), "https://github.com/valkey-io/valkey")}
				err := v.Set(ctx, fmt.Sprintf("links:%d", clientID), links)
				require.NoError(t, err)
			},
			test: func(clientID int64, s *application.CachedScrapperService) error {
				res, err := s.GetLinks(ctx, clientID)
				require.NoError(t, err)
				require.Len(t, res, 1)
				require.Equal(t, "https://github.com/valkey-io/valkey", res[0].Link())
				return nil
			},
		},
		{
			name:     "cache invalidation on subs",
			clientID: 3,
			prepare: func(clientID int64, v *cache.Valkey, tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository) {
				expectTx(tx)
				_ = v.Set(ctx, fmt.Sprintf("links:%d", clientID), []*domain.LinkSubscription{})
				users.EXPECT().GetClientByID(gomock.Any(), clientID).Return(domain.NewClient(clientID), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), gomock.Any()).Return(nil, application.ErrNotFound)
				links.EXPECT().AddLink(gomock.Any(), gomock.Any()).Return(nil)
				subs.EXPECT().AddSubscription(gomock.Any(), gomock.Any()).Return(nil)
			},
			test: func(clientID int64, s *application.CachedScrapperService) error {
				return s.Subscribe(ctx, "https://github.com/golang/go", clientID)
			},
			check: func(clientID int64, _ *domain.LinkSubscription, t *testing.T, v *cache.Valkey) {
				cacheKey := fmt.Sprintf("links:%d", clientID)
				_, ok, err := v.Get(ctx, cacheKey)
				require.NoError(t, err)
				require.False(t, ok)
			},
		},
		{
			name:     "cache invalidation on tags",
			clientID: 4,
			prepare: func(clientID int64, v *cache.Valkey, tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository, subs *mocks.MockSubscriptionsRepository) {
				expectTx(tx)
				_ = v.Set(ctx, fmt.Sprintf("links:%d", clientID), []*domain.LinkSubscription{})
				linkObj, _ := domain.NewTrackedLink("https://github.com/golangci/golangci-lint")
				users.EXPECT().GetClientByID(gomock.Any(), clientID).Return(domain.NewClient(clientID), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), gomock.Any()).Return(linkObj, nil)
				subs.EXPECT().AddTags(gomock.Any(), clientID, gomock.Any(), gomock.Any()).Return(nil)
			},
			test: func(clientID int64, s *application.CachedScrapperService) error {
				return s.AddTags(ctx, clientID, "https://github.com/golangci/golangci-lint", []string{"tag1"})
			},
			check: func(clientID int64, _ *domain.LinkSubscription, t *testing.T, v *cache.Valkey) {
				cacheKey := fmt.Sprintf("links:%d", clientID)
				_, ok, err := v.Get(ctx, cacheKey)
				require.NoError(t, err)
				require.False(t, ok)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(tt.clientID, valkeyClient, tx, userRepo, linksRepo, subsRepo)
			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, valkeyClient)

			err := tt.test(tt.clientID, cacheService)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.check != nil {
				tt.check(tt.clientID, tt.link, t, valkeyClient)
			}
		})
	}
}

func setupCache(ctx context.Context, t *testing.T) *cache.Valkey {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	t.Cleanup(cancel)

	container, err := valkey.Run(
		ctx,
		"valkey/valkey:9.0.3",
		valkey.WithSnapshotting(10, 1),
		valkey.WithLogLevel(valkey.LogLevelDebug),
		testcontainers.WithExposedPorts("6379/tcp"),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = container.Terminate(context.Background())
		require.NoError(t, err)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "6379")
	require.NoError(t, err)

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	codec := &cache.JSONCodec{}

	valkeyCfg := &cache.ValkeyConfig{
		Endpoints:  []string{host + ":" + port.Port()},
		MasterName: "",
		TTL:        5 * time.Minute,
	}

	m := &metricsMock{}

	client, err := cache.NewClient(logger, codec, m, valkeyCfg)
	require.NoError(t, err)

	return client
}

func expectTx(tx *mocks.MockTransactor) {
	tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, txFunc func(context.Context) error) error {
			return txFunc(ctx)
		})
}
