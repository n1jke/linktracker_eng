package application_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application/mocks"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

func expectTx(tx *mocks.MockTransactor) {
	tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, txFunc func(context.Context) error) error {
			return txFunc(ctx)
		})
}

func TestScrapperService_AddClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      int64
		prepare func(repo *mocks.MockUserRepository)
		wantErr bool
	}{
		{
			name: "valid",
			id:   1,
			prepare: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().AddClient(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name: "already exists",
			id:   2,
			prepare: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().AddClient(gomock.Any(), gomock.Any()).Return(application.ErrAlreadyExists)
			},
			wantErr: true,
		},
		{
			name: "error from storage",
			id:   3,
			prepare: func(repo *mocks.MockUserRepository) {
				repo.EXPECT().AddClient(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksCache := mocks.NewMockLinksCache(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(userRepo)

			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, linksCache)

			err := cacheService.AddClient(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScrapperService_GetLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      int64
		prepare func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
			subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache)
		wantErr bool
		wantLen int
	}{
		{
			name: "cache miss ok empty",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), int64(1)).Return(nil, application.ErrNotFound)
				cache.EXPECT().Set(gomock.Any(), "links:1", gomock.Any()).Return(nil)
			},
			wantLen: 0,
		},
		{
			name: "cache miss ok not empty",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)

				links := []*domain.LinkSubscription{
					domain.NewLinkSubscription(1, uuid.New(), "https://github.com/golang/go"),
				}
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), int64(1)).Return(links, nil)
				cache.EXPECT().Set(gomock.Any(), "links:1", gomock.Any()).Return(nil)
			},
			wantLen: 1,
		},
		{
			name: "cache miss chat not found",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				_ *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(nil, application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "cache miss storage error",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), int64(1)).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
		{
			name: "cache miss transaction fail",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, _ *mocks.MockUserRepository,
				_ *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			wantErr: true,
		},
		{
			name: "cache hit",
			id:   1,
			prepare: func(_ *mocks.MockTransactor, _ *mocks.MockUserRepository,
				_ *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				links := []*domain.LinkSubscription{domain.NewLinkSubscription(1, uuid.New(), "https://github.com/golang/go")}
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(links, true, nil)
			},
			wantLen: 1,
		},
		{
			name: "cache error fallback to db",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, assert.AnError)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), int64(1)).Return(nil, application.ErrNotFound)
				cache.EXPECT().Set(gomock.Any(), "links:1", gomock.Any()).Return(nil)
			},
			wantLen: 0,
		},
		{
			name: "cache set fail but ok",
			id:   1,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				cache.EXPECT().Get(gomock.Any(), "links:1").Return(nil, false, nil)
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				subs.EXPECT().GetSubscriptionsByUserID(gomock.Any(), int64(1)).Return(nil, application.ErrNotFound)
				cache.EXPECT().Set(gomock.Any(), "links:1", gomock.Any()).Return(assert.AnError)
			},
			wantLen: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksCache := mocks.NewMockLinksCache(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(tx, userRepo, subsRepo, linksCache)

			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, linksCache)

			links, err := cacheService.GetLinks(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, links, tt.wantLen)
			}
		})
	}
}

func TestScrapperService_RemoveClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      int64
		prepare func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
			subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache)
		wantErr bool
	}{
		{
			name: "ok",
			id:   21,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(domain.NewClient(21), nil)
				subs.EXPECT().RemoveSubscriptionsByUserID(gomock.Any(), int64(21)).Return(nil)
				users.EXPECT().RemoveClient(gomock.Any(), int64(21)).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:21").Return(nil)
			},
		},
		{
			name: "chat not found",
			id:   21,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				_ *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(nil, application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "remove subs error",
			id:   21,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(domain.NewClient(21), nil)
				subs.EXPECT().RemoveSubscriptionsByUserID(gomock.Any(), int64(21)).Return(assert.AnError)
			},
			wantErr: true,
		},
		{
			name: "remove client not found",
			id:   21,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(domain.NewClient(21), nil)
				subs.EXPECT().RemoveSubscriptionsByUserID(gomock.Any(), int64(21)).Return(nil)
				users.EXPECT().RemoveClient(gomock.Any(), int64(21)).Return(application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "cache invalidate fail but ok",
			id:   21,
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(domain.NewClient(21), nil)
				subs.EXPECT().RemoveSubscriptionsByUserID(gomock.Any(), int64(21)).Return(nil)
				users.EXPECT().RemoveClient(gomock.Any(), int64(21)).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:21").Return(assert.AnError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksCache := mocks.NewMockLinksCache(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(tx, userRepo, subsRepo, linksCache)

			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, linksCache)

			err := cacheService.RemoveClient(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScrapperService_Subscribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      int64
		link    string
		tags    []string
		prepare func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
			subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache)
		wantErr bool
	}{
		{
			name: "ok new link",
			id:   1,
			link: "https://github.com/golang/go",
			tags: []string{"work"},
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(nil, application.ErrNotFound)
				links.EXPECT().AddLink(gomock.Any(), gomock.Any()).Return(nil)
				subs.EXPECT().AddSubscription(gomock.Any(), gomock.Any()).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:1").Return(nil)
			},
		},
		{
			name: "already tracked",
			id:   21,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(21)).Return(domain.NewClient(21), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(nil, application.ErrNotFound)
				links.EXPECT().AddLink(gomock.Any(), gomock.Any()).Return(nil)
				subs.EXPECT().AddSubscription(gomock.Any(), gomock.Any()).Return(application.ErrAlreadyExists)
			},
			wantErr: true,
		},
		{
			name: "chat not found",
			id:   33,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, _ *mocks.MockLinksRepository,
				_ *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(33)).Return(nil, application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "invalid link",
			id:   33,
			link: "https://github.com/golang",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				_ *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(33)).Return(domain.NewClient(33), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang").Return(nil, application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "transaction fail",
			id:   1,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, _ *mocks.MockUserRepository, _ *mocks.MockLinksRepository,
				_ *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				tx.EXPECT().WithTransaction(gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			wantErr: true,
		},
		{
			name: "cache invalidate fail but ok",
			id:   1,
			link: "https://github.com/golang/go",
			tags: []string{"work"},
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(nil, application.ErrNotFound)
				links.EXPECT().AddLink(gomock.Any(), gomock.Any()).Return(nil)
				subs.EXPECT().AddSubscription(gomock.Any(), gomock.Any()).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:1").Return(assert.AnError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksCache := mocks.NewMockLinksCache(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(tx, userRepo, linksRepo, subsRepo, linksCache)

			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, linksCache)

			err := cacheService.Subscribe(context.Background(), tt.link, tt.id, tt.tags...)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestScrapperService_UnSubscribe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		id      int64
		link    string
		prepare func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
			subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache)
		wantErr bool
	}{
		{
			name: "ok",
			id:   1,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)

				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(linkObj, nil)
				subs.EXPECT().RemoveSubscription(gomock.Any(), gomock.Any()).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:1").Return(nil)
			},
		},
		{
			name: "link not found",
			id:   1,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				_ *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(nil, application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "subscription not found",
			id:   1,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, _ *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)

				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(linkObj, nil)
				subs.EXPECT().RemoveSubscription(gomock.Any(), gomock.Any()).Return(application.ErrNotFound)
			},
			wantErr: true,
		},
		{
			name: "cache invalidate fail but ok",
			id:   1,
			link: "https://github.com/golang/go",
			prepare: func(tx *mocks.MockTransactor, users *mocks.MockUserRepository, links *mocks.MockLinksRepository,
				subs *mocks.MockSubscriptionsRepository, cache *mocks.MockLinksCache,
			) {
				expectTx(tx)
				users.EXPECT().GetClientByID(gomock.Any(), int64(1)).Return(domain.NewClient(1), nil)

				linkObj, _ := domain.NewTrackedLink("https://github.com/golang/go")
				links.EXPECT().GetLinkByName(gomock.Any(), "https://github.com/golang/go").Return(linkObj, nil)
				subs.EXPECT().RemoveSubscription(gomock.Any(), gomock.Any()).Return(nil)
				cache.EXPECT().Delete(gomock.Any(), "links:1").Return(assert.AnError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			userRepo := mocks.NewMockUserRepository(ctrl)
			linksRepo := mocks.NewMockLinksRepository(ctrl)
			subsRepo := mocks.NewMockSubscriptionsRepository(ctrl)
			linksCache := mocks.NewMockLinksCache(ctrl)
			tx := mocks.NewMockTransactor(ctrl)

			tt.prepare(tx, userRepo, linksRepo, subsRepo, linksCache)

			s := application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)
			cacheService := application.NewCachedScrapperService(logger, s, linksCache)

			err := cacheService.UnSubscribe(context.Background(), tt.id, tt.link)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
