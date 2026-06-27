package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/n1jke/linktracker_eng/config"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

const (
	insertLinkQuery = `
    INSERT INTO links(link_id, path, resource_type)
    VALUES ($1, $2, $3)
  `

	insertUserQuery = `
    INSERT INTO users(chat_id)
    VALUES ($1)
  `

	insertSubsriptionQuery = `
    INSERT INTO subscriptions (user_id, res_id, link, tags, last_update)
    VALUES ($1, $2, $3, $4, $5)
  `
)

const (
	usersCount int64  = 1000
	linksCount int64  = 100
	linkSource string = "https://github.com/n1jke/"
)

func main() {
	ctx := context.Background()

	err := run(ctx)
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := ProvideConfig()
	if err != nil {
		return err
	}

	conn, err := ProvideDBConn(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	s := time.Now()

	err = fillTestDB(ctx, conn)
	if err != nil {
		return err
	}

	fmt.Printf("insert to db in %v\n", time.Since(s))

	return nil
}

func ProvideConfig() (*config.AppConfig, error) {
	return config.LoadConfig()
}

func ProvideDBConn(ctx context.Context, cfg *config.AppConfig) (*pgx.Conn, error) {
	connConfig, err := pgx.ParseConfig(cfg.DB.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("setup connection: %w", err)
	}

	return conn, nil
}

func fillTestDB(ctx context.Context, pool *pgx.Conn) error {
	batch := &pgx.Batch{}

	users := insertUsers(batch, usersCount)
	links := insertLinks(batch, linksCount)
	insertSubs(batch, users, links)

	batchResults := pool.SendBatch(ctx, batch)

	return batchResults.Close()
}

func insertUsers(batch *pgx.Batch, count int64) []*domain.Client {
	users := make([]*domain.Client, 0, count)

	for i := range count {
		users = append(users, domain.NewClient(i))
		batch.Queue(insertUserQuery, i)
	}

	return users
}

func insertLinks(batch *pgx.Batch, count int64) []*domain.TrackedLink {
	links := make([]*domain.TrackedLink, 0, count)

	for i := range count {
		link, err := domain.NewTrackedLink(fmt.Sprintf("%s%d", linkSource, i))
		if err != nil {
			log.Printf("fail to isert link: %v", err)
		}

		links = append(links, link)
		batch.Queue(insertLinkQuery, link.ID(), link.Path(), link.Type())
	}

	return links
}

func insertSubs(batch *pgx.Batch, users []*domain.Client, links []*domain.TrackedLink) {
	for i := range users {
		for j := range links {
			sub := domain.NewLinkSubscription(users[i].ID(), links[j].ID(), links[j].Path())
			batch.Queue(insertSubsriptionQuery, sub.UserID(), sub.ResourceID(), sub.Link(), sub.Tags(), sub.LastUpdate())
		}
	}
}
