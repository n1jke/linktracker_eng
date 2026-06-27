package fallback

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
	"github.com/n1jke/linktracker_eng/internal/scrapper/infrastructure/scheduler"
)

type Notifier struct {
	logger   *slog.Logger
	primary  scheduler.Notifier
	fallback scheduler.Notifier
}

func NewNotifier(logger *slog.Logger, primary, fallback scheduler.Notifier) *Notifier {
	return &Notifier{
		logger:   logger.With("module", "fallback-notifier"),
		primary:  primary,
		fallback: fallback,
	}
}

func (f *Notifier) SendUpdate(ctx context.Context, update *application.ResourceShot) error {
	err := f.primary.SendUpdate(ctx, update)
	if err == nil {
		return nil
	}

	f.logger.Warn("send msg primary", slog.Any("err", err), slog.Int64("id", update.ID))

	fallbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	// timeout лучше вынести в конфиг, но в след дз все равно уберем
	defer cancel()

	errFall := f.fallback.SendUpdate(fallbackCtx, update)
	if errFall != nil {
		f.logger.Warn("send msg fallback", slog.Any("err", errFall), slog.Int64("id", update.ID))
		return errors.Join(err, errFall)
	}

	return nil
}
