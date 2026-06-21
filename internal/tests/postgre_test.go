//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/domain"
	querybuilder "github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository/query_builder"
	sqlstorage "github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository/sql"
)

type metricsMock struct{}

func (m *metricsMock) Observe(_, _ string, _ time.Time) {}

type repoTemplate struct {
	name    string
	newUser func(pool *pgxpool.Pool) application.UserRepository
	newLink func(pool *pgxpool.Pool) application.LinksRepository
	newSubs func(pool *pgxpool.Pool) application.SubscriptionsRepository
}

const chatID int64 = 1214011736

func Test_Postgres(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)
	m := &metricsMock{}

	tests := []repoTemplate{
		{
			name: "sql",
			newUser: func(pool *pgxpool.Pool) application.UserRepository {
				return sqlstorage.NewUserRepoSQL(pool, m)
			},
			newLink: func(pool *pgxpool.Pool) application.LinksRepository {
				return sqlstorage.NewLinkRepoSQL(pool, m)
			},
			newSubs: func(pool *pgxpool.Pool) application.SubscriptionsRepository {
				return sqlstorage.NewSubscriptionsRepoSQL(pool, m)
			},
		},
		{
			name: "goqu",
			newUser: func(pool *pgxpool.Pool) application.UserRepository {
				return querybuilder.NewUserRepoGoqu(pool, m)
			},
			newLink: func(pool *pgxpool.Pool) application.LinksRepository {
				return querybuilder.NewLinkRepoGoqu(pool, m)
			},
			newSubs: func(pool *pgxpool.Pool) application.SubscriptionsRepository {
				return querybuilder.NewSubscriptionsRepoGoqu(pool, m)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			pool := setupDB(ctx, t)

			userRepo := tt.newUser(pool)
			linkRepo := tt.newLink(pool)
			subsRepo := tt.newSubs(pool)

			postgreTest(ctx, t, userRepo, linkRepo, subsRepo)
		})
	}
}

func postgreTest(ctx context.Context, t *testing.T,
	userRepo application.UserRepository,
	linkRepo application.LinksRepository,
	subsRepo application.SubscriptionsRepository,
) {
	// user repo
	err := userRepo.AddClient(ctx, domain.NewClient(chatID))
	require.NoError(t, err)

	err = userRepo.AddClient(ctx, domain.NewClient(chatID))
	require.ErrorIs(t, err, application.ErrAlreadyExists)

	gotClient, err := userRepo.GetClientByID(ctx, chatID)
	require.NoError(t, err)
	require.Equal(t, chatID, gotClient.ID())

	// link repo
	githubLink, err := domain.NewTrackedLink("https://github.com/uber-go/fx")
	require.NoError(t, err)

	stackLink, err := domain.NewTrackedLink("https://stackoverflow.com/questions/20778771/")
	require.NoError(t, err)

	err = linkRepo.AddLink(ctx, githubLink)
	require.NoError(t, err)

	err = linkRepo.AddLink(ctx, stackLink)
	require.NoError(t, err)

	err = linkRepo.AddLink(ctx, githubLink)
	require.ErrorIs(t, err, application.ErrAlreadyExists)

	gotLink, err := linkRepo.GetLinkByName(ctx, githubLink.Path())
	require.NoError(t, err)
	require.Equal(t, githubLink.ID(), gotLink.ID())

	// subs repo
	err = subsRepo.AddSubscription(ctx, domain.NewLinkSubscription(chatID, githubLink.ID(), githubLink.Path(), "work", "go"))
	require.NoError(t, err)

	err = subsRepo.AddSubscription(ctx, domain.NewLinkSubscription(chatID, stackLink.ID(), stackLink.Path()))
	require.NoError(t, err)

	err = subsRepo.AddSubscription(ctx, domain.NewLinkSubscription(chatID, githubLink.ID(), githubLink.Path(), "work", "go"))
	require.ErrorIs(t, err, application.ErrAlreadyExists)

	subs, err := subsRepo.GetSubscriptionsByUserID(ctx, chatID)
	require.NoError(t, err)
	require.Len(t, subs, 2)

	now := time.Now()
	err = subsRepo.UpdateSubscriptionLastUpdate(ctx, chatID, githubLink.ID(), now)
	require.NoError(t, err)
}

func setupDB(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	t.Cleanup(cancel)

	container, err := postgrescontainer.Run(ctx,
		"postgres:16-alpine",
		postgrescontainer.WithDatabase(t.Name()),
		postgrescontainer.WithUsername("scrapper"),
		postgrescontainer.WithPassword("scrapper"),
		postgrescontainer.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		err = container.Terminate(context.Background())
		require.NoError(t, err)
	})

	conn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, conn)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	migrateCtx, migrateCancel := context.WithTimeout(ctx, 3*time.Second)
	defer migrateCancel()

	_, err = pool.Exec(migrateCtx, migrationSQL)
	require.NoError(t, err)

	return pool
}

const migrationSQL = `
CREATE TABLE IF NOT EXISTS users (
  chat_id BIGINT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS links (
  link_id UUID PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  resource_type TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subscriptions (
  subscription_id SERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(chat_id),
  res_id UUID NOT NULL REFERENCES links(link_id),
  link TEXT NOT NULL,
  tags TEXT[] NOT NULL DEFAULT '{}',
  last_update TIMESTAMPTZ,
  UNIQUE (user_id, res_id)
);

CREATE INDEX IF NOT EXISTS index_chat_id ON users(chat_id);

CREATE INDEX IF NOT EXISTS index_link_id ON links(link_id);

CREATE INDEX IF NOT EXISTS index_subs_user_id ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS index_subs_res_id ON subscriptions(res_id);
`
