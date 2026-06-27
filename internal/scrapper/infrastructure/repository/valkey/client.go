package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/valkey-io/valkey-go"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

type MetricsRecorder interface {
	Observe(scope, scopeType string, start time.Time)
}

const scope = "cache"

type ValkeyConfig struct {
	Endpoints  []string
	MasterName string
	username   string
	password   string
	TTL        time.Duration
}

func NewValkeyConfig(endpoints []string, master, username, password string, ttl time.Duration) *ValkeyConfig {
	return &ValkeyConfig{
		Endpoints:  endpoints,
		MasterName: master,
		username:   username,
		password:   password,
		TTL:        ttl,
	}
}

type Codec interface {
	Encode(links []*domain.LinkSubscription) (string, error)
	Decode(data string) ([]*domain.LinkSubscription, error)
}

type Valkey struct {
	logger  *slog.Logger
	codec   Codec
	client  valkey.Client
	metrics MetricsRecorder
	ttl     time.Duration
}

func NewClient(logger *slog.Logger, codec Codec, m MetricsRecorder, cfg *ValkeyConfig) (*Valkey, error) {
	logger = logger.With("module", "valkey")

	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: cfg.Endpoints,
		Username:    cfg.username,
		Password:    cfg.password,
		Sentinel: valkey.SentinelOption{
			MasterSet: cfg.MasterName,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Valkey{
		logger:  logger,
		codec:   codec,
		client:  client,
		metrics: m,
		ttl:     cfg.TTL,
	}, nil
}

func (v *Valkey) Get(ctx context.Context, key string) ([]*domain.LinkSubscription, bool, error) {
	defer v.metrics.Observe(scope, "subscriptions", time.Now())

	resp := v.client.DoCache(ctx, v.client.B().Get().Key(key).Cache(), v.ttl)
	if err := resp.Error(); err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, false, nil
		}

		v.logger.Error("get", slog.String("key", key), slog.Any("err", err))

		return nil, false, err
	}

	data, err := resp.ToString()
	if err != nil {
		return nil, false, err
	}

	links, err := v.codec.Decode(data)
	if err != nil {
		v.logger.Error("decode", slog.Any("err", err))
		return nil, false, err
	}

	return links, true, nil
}

func (v *Valkey) Set(ctx context.Context, key string, links []*domain.LinkSubscription) error {
	defer v.metrics.Observe(scope, "subscriptions", time.Now())

	encoded, err := v.codec.Encode(links)
	if err != nil {
		v.logger.Error("encode", slog.Any("err", err))
		return err
	}

	err = v.client.Do(ctx, v.client.B().Set().Key(key).Value(encoded).Ex(v.ttl).Build()).Error()
	if err != nil {
		v.logger.Error("set", slog.String("key", key), slog.Any("err", err))
	}

	return err
}

func (v *Valkey) Delete(ctx context.Context, key string) error {
	defer v.metrics.Observe(scope, "subscriptions", time.Now())

	err := v.client.Do(ctx, v.client.B().Del().Key(key).Build()).Error()
	if err != nil {
		v.logger.Error("delete", slog.String("key", key), slog.Any("err", err))
	}

	return err
}

func (v *Valkey) Close() error {
	v.client.Close()
	return nil
}
