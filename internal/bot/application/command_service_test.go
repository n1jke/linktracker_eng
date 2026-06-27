package application_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/n1jke/linktracker_eng/internal/bot/application"
	"github.com/n1jke/linktracker_eng/internal/bot/application/mocks"
	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

func TestCommandUseCase_AvailableHandler(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		chatID  int64
		text    string
		prepare func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository)
		wantMsg string
		wantErr error
	}

	tests := []testCase{
		{
			name:   "start command",
			chatID: 1,
			text:   "/start",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(1)).Return(application.ChatState{
					ChatID: 1,
					State:  application.Available,
				}).Times(1)
				repo.EXPECT().Remove(int64(1)).Times(1)
				scrapper.EXPECT().RegisterChat(gomock.Any(), int64(1)).Return(nil)
			},
			wantMsg: application.StartMsg,
		},
		{
			name:   "unknown command",
			chatID: 0,
			text:   "/lumpa",
			prepare: func(_ *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(0)).Return(application.ChatState{
					ChatID: 0,
					State:  application.Available,
				}).Times(1)
			},
			wantMsg: application.DefaultMsg,
			wantErr: nil,
		},
		{
			name:   "untrack without url",
			chatID: 104,
			text:   "/untrack",
			prepare: func(_ *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(104)).Return(application.ChatState{
					ChatID: 104,
					State:  application.Available,
				}).Times(1)
			},
			wantMsg: application.UntrackUsageMsg,
		},
		{
			name:   "valid untrack",
			chatID: 105,
			text:   "/untrack https://github.com/uber-go/mock",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(105)).Return(application.ChatState{
					ChatID: 105,
					State:  application.Available,
				}).Times(1)
				scrapper.EXPECT().UntrackLink(gomock.Any(), int64(105), "https://github.com/uber-go/mock").Return(nil)
			},
			wantMsg: application.UntrackDoneMsg,
		},
		{
			name:   "empty list",
			chatID: 106,
			text:   "/list",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(106)).Return(application.ChatState{
					ChatID: 106,
					State:  application.Available,
				}).Times(1)
				scrapper.EXPECT().ListLinks(gomock.Any(), int64(106)).Return(nil, nil)
			},
			wantMsg: application.ListEmptyMsg,
		},
		{
			name:   "non empty list",
			chatID: 107,
			text:   "/list",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(107)).Return(application.ChatState{
					ChatID: 107,
					State:  application.Available,
				}).Times(1)

				links := []*domain.LinkSubscription{
					domain.NewLinkSubscription(107, uuid.New(), "https://github.com/uber-go/mock", "work", "go"),
					domain.NewLinkSubscription(107, uuid.New(), "http://stackoverflow.com/questions/20778771/", "edu"),
				}
				scrapper.EXPECT().ListLinks(gomock.Any(), int64(107)).Return(links, nil)
			},
			wantMsg: "Отслеживаемые ссылки:" +
				"\n1. https://github.com/uber-go/mock [tags: work, go]" +
				"\n2. http://stackoverflow.com/questions/20778771/ [tags: edu]",
		},
		{
			name:   "add tags",
			chatID: 109,
			text:   "/addtags https://github.com/uber-go/mock edu go",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(109)).Return(application.ChatState{
					ChatID: 109,
					State:  application.Available,
				}).Times(1)

				scrapper.EXPECT().
					AddTags(gomock.Any(), int64(109), "https://github.com/uber-go/mock", []string{"edu", "go"}).
					Return(nil).Times(1)
			},
			wantMsg: application.AddTagsDoneMsg,
		},
		{
			name:   "clear tags without url",
			chatID: 110,
			text:   "/cleartags",
			prepare: func(_ *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(110)).Return(application.ChatState{
					ChatID: 110,
					State:  application.Available,
				}).Times(1)
			},
			wantMsg: application.ClearTagsUsageMsg,
		},
		{
			name:   "valid clear tags",
			chatID: 111,
			text:   "/cleartags https://github.com/uber-go/mock",
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				repo.EXPECT().Get(int64(111)).Return(application.ChatState{
					ChatID: 111,
					State:  application.Available,
				}).Times(1)

				scrapper.EXPECT().
					ClearTags(gomock.Any(), int64(111), "https://github.com/uber-go/mock").
					Return(nil)
			},
			wantMsg: application.ClearTagsDoneMsg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			scrapper := mocks.NewMockScrapper(ctrl)
			states := mocks.NewMockChatStateRepository(ctrl)
			metrics := &metricsMock{}

			tt.prepare(scrapper, states)

			uc := application.NewCommandUseCase(states, scrapper, metrics)

			got, err := uc.Handle(context.Background(), tt.text, tt.chatID)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantMsg, got)
		})
	}
}

func TestCommandUseCase_FSM(t *testing.T) {
	t.Parallel()

	type step struct {
		text    string
		wantMsg string
		wantErr error
	}

	type testCase struct {
		name    string
		chatID  int64
		prepare func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository)
		steps   []step
	}

	tests := []testCase{
		{
			name:   "valid",
			chatID: 201,
			prepare: func(scrapper *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				gomock.InOrder(
					repo.EXPECT().Get(int64(201)).Return(application.ChatState{
						ChatID: 201,
						State:  application.Available,
					}),
					repo.EXPECT().StartTracking(int64(201)),

					repo.EXPECT().Get(int64(201)).Return(application.ChatState{
						ChatID: 201,
						State:  application.AwaitLink,
					}),
					repo.EXPECT().SetURL(int64(201), "https://github.com/uber-go/mock"),

					repo.EXPECT().Get(int64(201)).Return(application.ChatState{
						ChatID: 201,
						State:  application.AwaitTags,
						URL:    "https://github.com/uber-go/mock",
					}),
					repo.EXPECT().SetTags(int64(201), []string{"work", "go"}),

					repo.EXPECT().Get(int64(201)).Return(application.ChatState{
						ChatID: 201,
						State:  application.AwaitTags,
						URL:    "https://github.com/uber-go/mock",
						Tags:   []string{"work", "go"},
					}),

					scrapper.EXPECT().TrackLink(
						gomock.Any(),
						int64(201),
						"https://github.com/uber-go/mock",
						[]string{"work", "go"},
					).Return(nil),

					repo.EXPECT().Remove(int64(201)),
				)
			},
			steps: []step{
				{text: "/track", wantMsg: application.TrackStartMsg},
				{text: "/url https://github.com/uber-go/mock", wantMsg: application.TrackAwaitTagsMsg},
				{text: "/tags work go", wantMsg: application.TrackDoneMsg},
			},
		},
		{
			name:   "cancel and after try add url",
			chatID: 202,
			prepare: func(_ *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				gomock.InOrder(
					repo.EXPECT().Get(int64(202)).Return(application.ChatState{
						ChatID: 202,
						State:  application.Available,
					}),
					repo.EXPECT().StartTracking(int64(202)),

					repo.EXPECT().Get(int64(202)).Return(application.ChatState{
						ChatID: 202,
						State:  application.AwaitLink,
					}),
					repo.EXPECT().Remove(int64(202)),

					repo.EXPECT().Get(int64(202)).Return(application.ChatState{
						ChatID: 202,
						State:  application.Available,
					}),
				)
			},
			steps: []step{
				{text: "/track", wantMsg: application.TrackStartMsg},
				{text: "/cancel", wantMsg: application.TrackCancelMsg},
				{text: "/url https://github.com/uber-go/mock", wantMsg: application.DefaultMsg, wantErr: nil},
			},
		},
		{
			name:   "canceled(interrupted)",
			chatID: 203,
			prepare: func(_ *mocks.MockScrapper, repo *mocks.MockChatStateRepository) {
				gomock.InOrder(
					repo.EXPECT().Get(int64(203)).Return(application.ChatState{
						ChatID: 203,
						State:  application.Available,
					}),
					repo.EXPECT().StartTracking(int64(203)),

					repo.EXPECT().Get(int64(203)).Return(application.ChatState{
						ChatID: 203,
						State:  application.AwaitLink,
					}),
					repo.EXPECT().Remove(int64(203)),
				)
			},
			steps: []step{
				{text: "/track", wantMsg: application.TrackStartMsg},
				{text: "/help", wantMsg: application.TrackCancelMsg},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			scrapper := mocks.NewMockScrapper(ctrl)
			states := mocks.NewMockChatStateRepository(ctrl)
			metrics := &metricsMock{}

			tt.prepare(scrapper, states)

			uc := application.NewCommandUseCase(states, scrapper, metrics)

			for _, st := range tt.steps {
				got, err := uc.Handle(context.Background(), st.text, tt.chatID)
				if st.wantErr != nil {
					require.Error(t, err)
					assert.ErrorIs(t, err, st.wantErr)
				} else {
					require.NoError(t, err)
				}

				assert.Equal(t, st.wantMsg, got)
			}
		})
	}
}

type metricsMock struct{}

func (m *metricsMock) AddCommandRate(_ string) {}
