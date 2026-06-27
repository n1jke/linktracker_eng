package application_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/agent/application"
	"github.com/n1jke/linktracker_eng/internal/agent/application/mocks"
	"github.com/n1jke/linktracker_eng/internal/agent/domain"
)

const workerCount = 10

func TestAgentService_PrepareUpdate(t *testing.T) {
	t.Parallel()

	policy := domain.NewFilteringPolicy(&domain.FilterConfig{
		StopWords:       []string{"pupupu"},
		ExcludedAuthors: []domain.Author{"some"},
		MinLength:       10,
		Threshold:       50,
	})

	tests := []struct {
		name    string
		raw     *application.RawUpdate
		prepare func(*mocks.MockSummarize)
		wantNil bool
	}{
		{
			name:    "stop word in descr -> skip",
			raw:     &application.RawUpdate{Description: "pupupu makadf"},
			prepare: func(_ *mocks.MockSummarize) {},
			wantNil: true,
		},
		{
			name:    "excluded author -> skip",
			raw:     &application.RawUpdate{Author: "some"},
			prepare: func(_ *mocks.MockSummarize) {},
			wantNil: true,
		},
		{
			name:    "to short -> skip",
			raw:     &application.RawUpdate{Description: "hi"},
			prepare: func(_ *mocks.MockSummarize) {},
			wantNil: true,
		},
		{
			name: "pass filter",
			raw: &application.RawUpdate{
				ID: 1, Link: "github.com", Description: "normal length msg", Author: "n1jke", ChatIDs: []int64{1},
			},
			prepare: func(_ *mocks.MockSummarize) {},
		},
		{
			name: "summarize without error",
			raw: &application.RawUpdate{
				ID:          1,
				Link:        "github.com",
				Description: strings.Repeat("long msg ", 7),
				Author:      "n1jke",
				ChatIDs:     []int64{1},
			},
			prepare: func(s *mocks.MockSummarize) {
				s.EXPECT().Handle(gomock.Any(), gomock.Any()).Return(domain.Description("summarized text"), nil)
			},
		},
		{
			name: "summarize with error",
			raw: &application.RawUpdate{
				ID:          1,
				Link:        "github.com",
				Description: strings.Repeat("long msg ", 7),
				Author:      "n1jke",
				ChatIDs:     []int64{1},
			},
			prepare: func(s *mocks.MockSummarize) {
				s.EXPECT().Handle(gomock.Any(), gomock.Any()).Return(domain.Description(""), assert.AnError)
			},
		},
		{
			name: "avg text -> summarize",
			raw: &application.RawUpdate{
				ID: 1, Link: "github.com", Description: "normal pr msg", Author: "n1jke", ChatIDs: []int64{1},
			},
			prepare: func(_ *mocks.MockSummarize) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			summarizer := mocks.NewMockSummarize(ctrl)
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

			tt.prepare(summarizer)

			a := application.NewAgentService(logger, nil, policy, summarizer, workerCount)

			update := a.PrepareUpdate(context.Background(), tt.raw)
			if tt.wantNil {
				assert.Nil(t, update)
			} else {
				assert.NotNil(t, update)
			}
		})
	}
}

func TestAgentService_HandleUpdatesWindow(t *testing.T) {
	t.Parallel()

	policy := domain.NewFilteringPolicy(&domain.FilterConfig{
		HighPriority: []string{"crit"},
		LowPriority:  []string{"typo"},
		MinLength:    5,
		Threshold:    1000,
	})

	tests := []struct {
		name    string
		prepare func(outbox *mocks.MockOutboxRepository, summirizer *mocks.MockSummarize)
		updates []*application.RawUpdate
		wantErr bool
		check   func(*testing.T, []*application.ProcessedUpdate)
	}{
		{
			name: "group updates",
			prepare: func(outbox *mocks.MockOutboxRepository, _ *mocks.MockSummarize) {
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Do(
					func(_ context.Context, updates []*application.ProcessedUpdate) {
						require.Len(t, updates, 1)
						assert.Equal(t, []int64{2}, updates[0].ChatIDs)
						assert.Equal(t, domain.High, updates[0].Priority)
					},
				).Return(nil)
			},
			updates: []*application.RawUpdate{
				{ID: 1, Link: "url1", Description: "feat: some update", Author: "n1jke", ChatIDs: []int64{2}},
				{ID: 2, Link: "url1", Description: "fix: fix crit issue", Author: "n1jke", ChatIDs: []int64{2}},
			},
		},
		{
			name: "single update",
			prepare: func(outbox *mocks.MockOutboxRepository, _ *mocks.MockSummarize) {
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Do(
					func(_ context.Context, updates []*application.ProcessedUpdate) {
						require.Len(t, updates, 1)
						assert.Equal(t, []int64{2}, updates[0].ChatIDs)
						assert.Equal(t, domain.Low, updates[0].Priority)
					},
				).Return(nil)
			},
			updates: []*application.RawUpdate{
				{ID: 1, Link: "url1", Description: "chore: typo fixes", Author: "n1jke", ChatIDs: []int64{2}},
			},
		},
		{
			name: "mixed priorities with gropping",
			prepare: func(outbox *mocks.MockOutboxRepository, _ *mocks.MockSummarize) {
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Do(
					func(_ context.Context, updates []*application.ProcessedUpdate) {
						require.Len(t, updates, 2)

						for _, upd := range updates {
							if upd.ID == 3 {
								assert.Equal(t, domain.Low, upd.Priority)
							} else {
								assert.Equal(t, domain.High, upd.Priority)
							}
						}
					},
				).Return(nil)
			},
			updates: []*application.RawUpdate{
				{ID: 1, Link: "url1", Description: "feat: crit security issue", Author: "n1jke", ChatIDs: []int64{111}},
				{ID: 2, Link: "url2", Description: "docs: update adr", Author: "n1jke", ChatIDs: []int64{111}},
				{ID: 3, Link: "url3", Description: "docs: update typo", Author: "n1jke", ChatIDs: []int64{222}},
			},
		},
		{
			name: "single and grouped chats",
			prepare: func(outbox *mocks.MockOutboxRepository, _ *mocks.MockSummarize) {
				outbox.EXPECT().AddBatch(gomock.Any(), gomock.Any()).Do(
					func(_ context.Context, updates []*application.ProcessedUpdate) {
						require.Len(t, updates, 2)

						for _, upd := range updates {
							if upd.ID == 3 || upd.ID == 2 {
								assert.Equal(t, []int64{222}, upd.ChatIDs)
								assert.Equal(t, domain.High, upd.Priority)
							} else {
								assert.Equal(t, []int64{111}, upd.ChatIDs)
								assert.Equal(t, domain.High, upd.Priority)
							}
						}
					},
				).Return(nil)
			},
			updates: []*application.RawUpdate{
				{ID: 1, Link: "url1", Description: "chore: some crit renames", Author: "n1jke", ChatIDs: []int64{111}},
				{ID: 2, Link: "url2", Description: "chore: some crit renames", Author: "n1jke", ChatIDs: []int64{222}},
				{ID: 3, Link: "url3", Description: "docs: some docs added", Author: "n1jke", ChatIDs: []int64{222}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			outbox := mocks.NewMockOutboxRepository(ctrl)
			summarizer := mocks.NewMockSummarize(ctrl)

			tt.prepare(outbox, summarizer)

			a := application.NewAgentService(logger, outbox, policy, summarizer, workerCount)

			err := a.HandleUpdatesWindow(context.Background(), tt.updates)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMaxPriority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		updates []*application.ProcessedUpdate
		want    domain.Priority
	}{
		{
			name: "all low",
			updates: []*application.ProcessedUpdate{
				{Priority: domain.Low},
				{Priority: domain.Low},
			},
			want: domain.Low,
		},
		{
			name: "medium > low",
			updates: []*application.ProcessedUpdate{
				{Priority: domain.Low},
				{Priority: domain.Medium},
			},
			want: domain.Medium,
		},
		{
			name: "high > medium",
			updates: []*application.ProcessedUpdate{
				{Priority: domain.Low},
				{Priority: domain.Medium},
				{Priority: domain.High},
			},
			want: domain.High,
		},
		{
			name: "single",
			updates: []*application.ProcessedUpdate{
				{Priority: domain.High},
			},
			want: domain.High,
		},
		{
			name:    "defaults",
			updates: []*application.ProcessedUpdate{},
			want:    domain.Low,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := application.MaxPriority(tt.updates)

			assert.Equal(t, tt.want, got)
		})
	}
}
