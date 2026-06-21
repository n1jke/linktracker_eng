// todo: move to fx.Modules(examples - docs, Yuriy Dzyuban seminars)
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/n1jke/linktracker/config"
	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/infrastructure/server"
	"github.com/n1jke/linktracker/internal/infrastructure/transport"
	transportgrpc "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc"
	interceptors "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc/interceptor"
	"github.com/n1jke/linktracker/internal/infrastructure/transport/http/middleware"
	scrapperhttp "github.com/n1jke/linktracker/internal/infrastructure/transport/http/scrapper"
	"github.com/n1jke/linktracker/internal/scrapper/application"
	"github.com/n1jke/linktracker/internal/scrapper/infrastructure/crawlers"
	grpcclient "github.com/n1jke/linktracker/internal/scrapper/infrastructure/grpc/client"
	grpcserver "github.com/n1jke/linktracker/internal/scrapper/infrastructure/grpc/server"
	httpclient "github.com/n1jke/linktracker/internal/scrapper/infrastructure/http/client"
	httpserver "github.com/n1jke/linktracker/internal/scrapper/infrastructure/http/server"
	producer "github.com/n1jke/linktracker/internal/scrapper/infrastructure/kafka"
	qb "github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository/query_builder"
	rawsql "github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository/sql"
	cache "github.com/n1jke/linktracker/internal/scrapper/infrastructure/repository/valkey"
	"github.com/n1jke/linktracker/internal/scrapper/infrastructure/scheduler"
	"github.com/n1jke/linktracker/internal/scrapper/infrastructure/telemetry"
	"github.com/n1jke/linktracker/pkg"
)

func NewApp() fx.Option {
	return fx.Options(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			ProvideDBPool,
			ProvideValkeyCache,
			ProvideRepositories,
			ProvideSchedulerOutboxRepository,
			ProvideCrawler,
			ProvideScheduler,
			ProvideClient,
			ProvideServer,
			ProvideCrawlerService,
			ProvideScrapperService,
		),
		sharedrepo.Module,
		telemetry.Module,
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log}
		}),
		fx.Invoke(RegisterDBLifecycle),
		fx.Invoke(RegisterScrapperLifecycle),
		fx.StartTimeout(time.Second*10),
	)
}

func ProvideConfig() (*config.AppConfig, error) {
	return config.LoadConfig()
}

func ProvideLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func ProvideCrawler(cfg *config.AppConfig, m crawlers.MetricsRecorder) application.SourceCrawler {
	return crawlers.NewWebCrawler(cfg.Scrapper.GithubToken, m)
}

func ProvideCrawlerService(cfg *config.AppConfig, logger *slog.Logger, tx application.Transactor,
	subsRepo application.SubscriptionsRepository, linksRepo application.LinksRepository,
	outboxRepo application.OutboxRepository, crawler application.SourceCrawler,
	m application.MetricsRecorder,
) *application.CrawlerService {
	return application.NewCrawlerService(logger, tx, subsRepo, linksRepo, outboxRepo,
		crawler, m, cfg.Scrapper.WorkerCount, cfg.Scrapper.BatchSize)
}

func ProvideScheduler(cfg *config.AppConfig, logger *slog.Logger, crawler *application.CrawlerService,
	repo scheduler.OutboxRepository, tx scheduler.Transactor, notifier scheduler.Notifier,
	pusher scheduler.Pusher,
) (scheduler.Runner, error) {
	schedulerCfg := cfg.MapSchedulerConfig()

	return scheduler.NewScheduleUpdates(logger, crawler, repo, tx, notifier, pusher, *schedulerCfg)
}

func ProvideDBPool(cfg *config.AppConfig) (*pgxpool.Pool, error) {
	connConfig, err := pgxpool.ParseConfig(cfg.DB.ConnectionString())
	if err != nil {
		return nil, err
	}

	connConfig.MaxConns = 5
	connConfig.MinConns = 1
	connConfig.MaxConnIdleTime = 500 * time.Millisecond
	connConfig.MaxConnLifetime = 10 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func ProvideValkeyCache(cfg *config.AppConfig, logger *slog.Logger, m cache.MetricsRecorder) (application.LinksCache, error) {
	if !cfg.Valkey.Enabled {
		return nil, nil
	}

	codec := &cache.JSONCodec{}
	cacheCfg := cfg.MapValkeyConfig()

	valkey, err := cache.NewClient(logger, codec, m, cacheCfg)
	if err != nil {
		return nil, err
	}

	return valkey, nil
}

func ProvideScrapperService(logger *slog.Logger, subsRepo application.SubscriptionsRepository, linksRepo application.LinksRepository,
	userRepo application.UserRepository, tx application.Transactor, linksCache application.LinksCache,
) application.LinkService {
	var service application.LinkService = application.NewScrapperService(logger, subsRepo, linksRepo, userRepo, tx)

	if linksCache != nil {
		service = application.NewCachedScrapperService(logger, service, linksCache)
	}

	return service
}

func ProvideRepositories(cfg *config.AppConfig, pool *pgxpool.Pool, rawM rawsql.MetricsRecorder, qbM qb.MetricsRecorder) (
	application.SubscriptionsRepository, application.LinksRepository, application.UserRepository, application.OutboxRepository,
) {
	if cfg.DB.Access == config.AccessSQL {
		return rawsql.NewSubscriptionsRepoSQL(pool, rawM), rawsql.NewLinkRepoSQL(pool, rawM),
			rawsql.NewUserRepoSQL(pool, rawM), rawsql.NewOutboxRepoSQL(pool, rawM)
	}

	return qb.NewSubscriptionsRepoGoqu(pool, qbM), qb.NewLinkRepoGoqu(pool, qbM), qb.NewUserRepoGoqu(pool, qbM),
		qb.NewOutboxRepoGoqu(pool, qbM)
}

func ProvideSchedulerOutboxRepository(cfg *config.AppConfig, pool *pgxpool.Pool, rawM rawsql.MetricsRecorder,
	qbM qb.MetricsRecorder,
) scheduler.OutboxRepository {
	if cfg.DB.Access == config.AccessSQL {
		return rawsql.NewOutboxRepoSQL(pool, rawM)
	}

	return qb.NewOutboxRepoGoqu(pool, qbM)
}

func ProvideClient(cfg *config.AppConfig, logger *slog.Logger) (scheduler.Notifier, io.Closer, error) {
	switch cfg.Communication.NotificationProtocol {
	case config.ProtocolHTTP:
		notifier, err := httpclient.NewBotClient(cfg.Bot.BotURL, logger, cfg.Timeouts.ScrapperClientReq, cfg.MapScrapperRetryConfig(),
			cfg.Scrapper.RetryHTTPCodes, cfg.MapBreakerConfig("scrapper-cb"))
		if err != nil {
			return nil, nil, err
		}

		return notifier, nil, nil
	case config.ProtocolGRPC:
		botAddr, err := pkg.ResolveAddr(cfg.Bot.BotURL)
		if err != nil {
			return nil, nil, err
		}

		conn, err := grpc.NewClient(botAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, nil, err
		}

		notifier := grpcclient.NewBotGRPCClient(conn, logger, cfg.Timeouts.ScrapperClientReq, cfg.MapScrapperRetryConfig(),
			cfg.Scrapper.RetryGRPCCodes, cfg.MapBreakerConfig("scrapper-cb"))

		return notifier, conn, nil
	case config.ProtocolKafka:
		notifier, err := producer.NewKafkaProducer(logger, cfg.MapProducerConfig())
		if err != nil {
			return nil, nil, err
		}

		return notifier, notifier, nil
	default:
		return nil, nil, fmt.Errorf("unsupported updates protocol: %s", cfg.Communication.NotificationProtocol)
	}
}

func ProvideServer(cfg *config.AppConfig, logger *slog.Logger, service application.LinkService, m transport.RateMetrics,
) (server.Runner, error) {
	scrapperAddr, err := pkg.ResolveAddr(cfg.Scrapper.ScrapperURL)
	if err != nil {
		return nil, err
	}

	switch cfg.Communication.RequestProtocol {
	case config.ProtocolHTTP:
		svr := httpserver.NewScrapperServer(service)
		handler := scrapperhttp.HandlerWithOptions(svr, scrapperhttp.StdHTTPServerOptions{
			Middlewares: []scrapperhttp.MiddlewareFunc{
				scrapperhttp.MiddlewareFunc(middleware.LogMiddleware(logger.With("component", "http_middleware"))),
				scrapperhttp.MiddlewareFunc(middleware.LimitMiddleware(cfg.Scrapper.RPS, cfg.Scrapper.Burst)),
				scrapperhttp.MiddlewareFunc(middleware.RateMiddleware(m)),
			},
		})

		return server.NewHTTPServer(scrapperAddr, handler,
			cfg.Timeouts.HTTPRead, cfg.Timeouts.HTTPWrite, cfg.Timeouts.HTTPIdle), nil
	case config.ProtocolGRPC:
		opt := make([]grpc.ServerOption, 0, 2)
		opt = append(opt, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: cfg.Timeouts.GRPCMaxConnIdle,
			Timeout:           cfg.Timeouts.GRPCKeepAlive,
		}), grpc.ChainUnaryInterceptor(
			interceptors.UnaryLimitInterceptor(cfg.Scrapper.RPS, cfg.Scrapper.Burst),
			interceptors.UnaryRateInterceptor(m),
		))

		return server.NewGRPCServer(scrapperAddr, opt, func(s *grpc.Server) {
			transportgrpc.RegisterScrapperServiceServer(s, grpcserver.NewScrapperGRPCServer(service))
		}), nil
	case config.ProtocolKafka:
		return nil, fmt.Errorf("unsupported api protocol: %s", cfg.Communication.RequestProtocol)
	default:
		return nil, fmt.Errorf("unsupported api protocol: %s", cfg.Communication.RequestProtocol)
	}
}

func RegisterDBLifecycle(lc fx.Lifecycle, pool *pgxpool.Pool) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			return nil
		},
		OnStop: func(context.Context) error {
			pool.Close()
			return nil
		},
	})
}

func RegisterScrapperLifecycle(lc fx.Lifecycle, logger *slog.Logger,
	runner server.Runner, crawlerScheduler scheduler.Runner, closer io.Closer, cfg *config.AppConfig,
) {
	var cancel context.CancelFunc

	done := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context

			ctx, cancel = context.WithCancel(context.Background())

			if err := crawlerScheduler.Start(ctx); err != nil {
				return err
			}

			go func() {
				defer close(done)

				if err := runner.Start(ctx); err != nil {
					logger.Error("scrapper svr crashed", slog.Any("err", err))
					cancel()
				}
			}()

			logger.Info("scrapper started")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("scrapper stopping")

			ctxShutdown, cancelShutdown := context.WithTimeout(ctx, cfg.Timeouts.Shutdown)
			defer cancelShutdown()

			if err := runner.Stop(ctxShutdown); err != nil {
				logger.Error("graceful stop svr", slog.Any("err", err))
			}

			if err := crawlerScheduler.Stop(); err != nil {
				logger.Error("stop scheduler", slog.Any("err", err))
			}

			cancel()

			select {
			case <-done:
				logger.Info("scrapper stopped")
			case <-ctxShutdown.Done():
				logger.Warn("scrapper stop deadline exceeded", slog.Any("err", ctxShutdown.Err()))
			}

			if err := closer.Close(); err != nil {
				logger.Error("close resource", slog.Any("err", err))
			}

			return nil
		},
	})
}
