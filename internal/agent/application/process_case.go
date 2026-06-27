package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/n1jke/linktracker_eng/internal/agent/domain"
)

const previewLimit = 200

//go:generate mockgen -source process_case.go -destination=mocks/mocks.go -package=mocks
type OutboxRepository interface {
	AddBatch(ctx context.Context, updates []*ProcessedUpdate) error
}

type Summarize interface {
	Handle(ctx context.Context, d domain.Description) (domain.Description, error)
}

type AgentService struct {
	logger      *slog.Logger
	outbox      OutboxRepository
	policy      *domain.FilteringPolicy
	summarizer  Summarize
	workerCount int
}

func NewAgentService(logger *slog.Logger, outbox OutboxRepository, policy *domain.FilteringPolicy,
	summarizer Summarize, workerCount int,
) *AgentService {
	logger = logger.With("module", "agent-service")

	return &AgentService{
		logger:      logger,
		outbox:      outbox,
		policy:      policy,
		summarizer:  summarizer,
		workerCount: workerCount,
	}
}

func (a *AgentService) PrepareUpdate(ctx context.Context, raw *RawUpdate) *ProcessedUpdate {
	e := raw.ToEvent()

	decision := a.policy.CheckEvent(e)
	descr := e.Description()

	switch decision.Action {
	case domain.Ignore:
		a.logger.Info("message ignored", slog.Int64("id", int64(e.ID())))
		return nil
	case domain.Pass:
	case domain.Summarize:
		var err error

		descr, err = a.summarizer.Handle(ctx, e.Description())
		if err != nil {
			a.logger.Warn("fail summarize description, move to fallback", slog.Int64("id", int64(e.ID())), slog.Any("err", err))

			descr = trimPreview(descr)
		}
	}

	send := e.WithSummarizedDescription(descr)
	processed := UpdateFromEvent(send, raw.EventType, raw.UpdatedAt, raw.ChatIDs, decision.Priority)

	a.logger.Info("event prepared", slog.Int64("id", int64(send.ID())))

	return processed
}

func (a *AgentService) HandleUpdatesWindow(ctx context.Context, updates []*RawUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	processed := a.processUpdates(ctx, updates)

	groups := make(map[int64][]*ProcessedUpdate, len(processed)) // key - chat_id
	for i := range processed {
		for _, id := range processed[i].ChatIDs {
			groups[id] = append(groups[id], processed[i])
		}
	}

	batch := make(map[int64][]int64, len(processed)) // key - event_id, value - chats with 1 update
	for id, upds := range groups {
		if len(upds) == 1 {
			batch[upds[0].ID] = append(batch[upds[0].ID], id)
		}
	}

	deliver := make([]*ProcessedUpdate, 0, len(groups))
	for id, upds := range groups {
		if len(upds) == 1 {
			if ids, ok := batch[upds[0].ID]; ok {
				// мутирую shared объект, но это не влияет ни на что, т.к
				// 1) те кто берет несколько обновлений не использую ChatIDs 2) все остальные идут в этом батче
				upds[0].ChatIDs = ids
				deliver = append(deliver, upds[0])
				delete(batch, upds[0].ID)
			}

			continue
		}

		group := make([]*ProcessedUpdate, 0, len(upds))
		group = append(group, upds...)

		deliver = append(deliver, GroupUpdates(group, id))
	}

	if len(deliver) == 0 {
		return nil
	}

	if err := a.outbox.AddBatch(ctx, deliver); err != nil {
		a.logger.Error("add batch to outbox", slog.Any("err", err))
		return fmt.Errorf("add batch to outbox: %w", err)
	}

	a.logger.Info("successful processed window batch")

	return nil
}

func (a *AgentService) processUpdates(ctx context.Context, updates []*RawUpdate) []*ProcessedUpdate {
	wg := new(sync.WaitGroup)
	jobs := make(chan *RawUpdate)
	prep := make(chan *ProcessedUpdate, len(updates))

	for range a.workerCount {
		wg.Go(func() {
			for e := range jobs {
				p := a.PrepareUpdate(ctx, e)
				if p != nil {
					prep <- p
				}
			}
		})
	}

	wg.Go(func() {
		defer close(jobs)

		for i := range updates {
			select {
			case jobs <- updates[i]:
			case <-ctx.Done():
				a.logger.Warn("context done")
				return
			}
		}
	})

	wg.Wait()
	close(prep)

	processed := make([]*ProcessedUpdate, 0, len(updates))
	for p := range prep {
		processed = append(processed, p)
	}

	return processed
}

func trimPreview(d domain.Description) domain.Description {
	s := strings.TrimSpace(string(d))
	if len(s) <= previewLimit {
		return d
	}

	return domain.Description(strings.TrimSpace(s[:previewLimit]))
}
