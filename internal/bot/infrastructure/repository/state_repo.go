package repository

import (
	"sync"

	"github.com/n1jke/linktracker/internal/bot/application"
)

type InMemoryChatStateRepository struct {
	mu    sync.RWMutex
	state map[int64]application.ChatState
}

func NewInMemoryChatStateRepository() *InMemoryChatStateRepository {
	return &InMemoryChatStateRepository{
		state: make(map[int64]application.ChatState),
	}
}

func (r *InMemoryChatStateRepository) Get(chatID int64) application.ChatState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.state[chatID]
	if !ok {
		return application.ChatState{
			ChatID: chatID,
			State:  application.Available,
		}
	}

	return s
}

func (r *InMemoryChatStateRepository) StartTracking(chatID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.state[chatID] = application.ChatState{
		ChatID: chatID,
		State:  application.AwaitLink,
	}
}

func (r *InMemoryChatStateRepository) SetURL(chatID int64, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.state[chatID]
	s.ChatID = chatID
	s.URL = url
	s.State = application.AwaitTags
	r.state[chatID] = s
}

func (r *InMemoryChatStateRepository) SetTags(chatID int64, tags []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.state[chatID]
	s.ChatID = chatID

	s.Tags = append([]string(nil), tags...)
	r.state[chatID] = s
}

func (r *InMemoryChatStateRepository) Remove(chatID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.state, chatID)
}
