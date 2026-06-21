// todo: move to fx.Modules(examples - docs, Yuriy Dzyuban seminars)
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/n1jke/linktracker/config"
	"github.com/n1jke/linktracker/internal/bot/application"
	grpcclient "github.com/n1jke/linktracker/internal/bot/infrastructure/grpc/client"
	botgrpcserver "github.com/n1jke/linktracker/internal/bot/infrastructure/grpc/server"
	httpclient "github.com/n1jke/linktracker/internal/bot/infrastructure/http/client"
	bothttpserver "github.com/n1jke/linktracker/internal/bot/infrastructure/http/server"
	consumer "github.com/n1jke/linktracker/internal/bot/infrastructure/kafka"
	"github.com/n1jke/linktracker/internal/bot/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/bot/infrastructure/scheduler"
	"github.com/n1jke/linktracker/internal/bot/infrastructure/telegram"
	"github.com/n1jke/linktracker/internal/bot/infrastructure/telemetry"
	sharedrepo "github.com/n1jke/linktracker/internal/infrastructure/repository"
	"github.com/n1jke/linktracker/internal/infrastructure/server"
	"github.com/n1jke/linktracker/internal/infrastructure/transport"
	transportgrpc "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc"
	interceptors "github.com/n1jke/linktracker/internal/infrastructure/transport/grpc/interceptor"
	bothttp "github.com/n1jke/linktracker/internal/infrastructure/transport/http/bot"
	"github.com/n1jke/linktracker/internal/infrastructure/transport/http/middleware"
	"github.com/n1jke/linktracker/pkg"
	"github.com/n1jke/linktracker/pkg/retry"
)

func NewApp() fx.Option {
	return fx.Options(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			ProvideClient,
			ProvideServer,
			ProvideTelegramBot,
			fx.Annotate( // https://uber-go.github.io/fx/annotate.html#casting-structs-to-interfaces
				application.NewCommandUseCase,
				fx.As(new(telegram.CommandService)),
			),
			fx.Annotate(
				func(s *telegram.Sender) scheduler.BotNotifier { return s.Bot() },
				fx.As(new(scheduler.BotNotifier)),
			),
		),
		sharedrepo.Module,
		repository.Module,
		scheduler.Module,
		telemetry.Module,
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			return &fxevent.SlogLogger{Logger: log}
		}),
		fx.Invoke(BotLifecycle),
	)
}

func ProvideBotConfig(cfg *config.AppConfig) *config.BotConfig {
	return &cfg.Bot
}

func ProvideTelegramBot(cfg *config.AppConfig, service telegram.CommandService, logger *slog.Logger) (*telegram.Sender, error) {
	r := retry.NewRetrier(cfg.MapBotRetryConfig())

	return telegram.NewTelegramBot(&cfg.Bot, service, r, logger)
}

func ProvideConfig() (*config.AppConfig, error) {
	return config.LoadConfig()
}

func ProvideLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func ProvideClient(cfg *config.AppConfig, logger *slog.Logger, m transport.DurationMetrics) (application.Scrapper, io.Closer, error) {
	switch cfg.Communication.RequestProtocol {
	case config.ProtocolHTTP:
		client, err := httpclient.NewScrapperClient(cfg.Scrapper.ScrapperURL, logger, cfg.Timeouts.BotClientReq, cfg.MapBotRetryConfig(),
			cfg.Bot.RetryHTTPCodes, cfg.MapBreakerConfig("bot-cb"), m)
		if err != nil {
			return nil, nil, err
		}

		return client, nil, nil
	case config.ProtocolGRPC:
		scrapperAddr, err := pkg.ResolveAddr(cfg.Scrapper.ScrapperURL)
		if err != nil {
			return nil, nil, err
		}

		conn, err := grpc.NewClient(scrapperAddr, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(interceptors.ClientUnaryDurationInterceptor(m)))
		if err != nil {
			return nil, nil, err
		}

		client := grpcclient.NewScrapperGRPCClient(conn, logger, cfg.Timeouts.BotClientReq, cfg.MapBotRetryConfig(),
			cfg.Bot.RetryGRPCCodes, cfg.MapBreakerConfig("bot-cb"))

		return client, conn, nil
	case config.ProtocolKafka:
		return nil, nil, fmt.Errorf("kafka is unsupported for requests")
	default:
		return nil, nil, fmt.Errorf("unsupported api protocol: %s", cfg.Communication.RequestProtocol)
	}
}

func ProvideServer(cfg *config.AppConfig, logger *slog.Logger, tgBot *telegram.Sender,
	inbox consumer.InboxRepository,
) (server.Runner, error) {
	botAddr, err := pkg.ResolveAddr(cfg.Bot.BotURL)
	if err != nil {
		return nil, err
	}

	switch cfg.Communication.NotificationProtocol {
	case config.ProtocolHTTP:
		svr := bothttpserver.NewBotServer(tgBot.Bot())
		handler := bothttp.HandlerWithOptions(svr, bothttp.StdHTTPServerOptions{
			Middlewares: []bothttp.MiddlewareFunc{
				bothttp.MiddlewareFunc(middleware.LogMiddleware(logger.With("component", "http_middleware"))),
			},
		})

		return server.NewHTTPServer(botAddr, handler,
			cfg.Timeouts.HTTPRead, cfg.Timeouts.HTTPWrite, cfg.Timeouts.HTTPIdle), nil
	case config.ProtocolGRPC:
		opt := make([]grpc.ServerOption, 0, 1)
		opt = append(opt, grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: cfg.Timeouts.GRPCMaxConnIdle,
			Timeout:           cfg.Timeouts.GRPCKeepAlive,
		}))

		return server.NewGRPCServer(botAddr, opt, func(s *grpc.Server) {
			transportgrpc.RegisterBotServiceServer(s, botgrpcserver.NewBotGRPCServer(tgBot.Bot()))
		}), nil
	case config.ProtocolKafka:
		return consumer.NewKafkaConsumer(logger, cfg.MapConsumerConfig(), inbox)
	default:
		return nil, fmt.Errorf("unsupported updates protocol: %s", cfg.Communication.NotificationProtocol)
	}
}

func BotLifecycle(lc fx.Lifecycle, logger *slog.Logger,
	runner server.Runner, tgBot *telegram.Sender, closer io.Closer, cfg *config.AppConfig,
) {
	var cancel context.CancelFunc

	done := make(chan struct{})

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			var ctx context.Context

			ctx, cancel = context.WithCancel(context.Background())

			if err := tgBot.Setup(ctx); err != nil {
				cancel()
				return err
			}

			go func() {
				if err := runner.Start(ctx); err != nil {
					logger.Error("bot svr crashed", slog.Any("err", err))
					cancel()
				}
			}()

			go func() {
				defer close(done)

				tgBot.Start(ctx)
			}()

			logger.Info("telegram bot started")

			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("telegram bot stopping")

			ctxShutdown, cancelShutdown := context.WithTimeout(ctx, cfg.Timeouts.Shutdown)
			defer cancelShutdown()

			if err := runner.Stop(ctxShutdown); err != nil {
				logger.Error("graceful stop svr", slog.Any("err", err))
			}

			cancel()

			select {
			case <-done:
				logger.Info("telegram bot stopped")
			case <-ctxShutdown.Done():
				logger.Warn("telegram bot stop deadline exceeded", slog.Any("err", ctxShutdown.Err()))
			}

			if closer != nil {
				if err := closer.Close(); err != nil {
					logger.Error("close resource", slog.Any("err", err))
				}
			}

			return nil
		},
	})
}
