package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-telegram/bot"

	bothttp "github.com/n1jke/linktracker_eng/internal/infrastructure/transport/http/bot"
)

type BotServer struct {
	server *bot.Bot
}

func NewBotServer(tBot *bot.Bot) *BotServer {
	return &BotServer{
		server: tBot,
	}
}

func (bs *BotServer) PostUpdates(w http.ResponseWriter, r *http.Request) {
	var req bothttp.LinkUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body")
		return
	}

	if req.Description == nil || req.Url == nil || req.TgChatIds == nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body params")
		return
	}

	unsend := 0

	for _, chatID := range *req.TgChatIds {
		_, err := bs.server.SendMessage(r.Context(), &bot.SendMessageParams{
			ChatID: chatID,
			Text:   *req.Description,
		})
		if err != nil {
			unsend++
		}
	}

	if unsend > 0 {
		writeErr(w, http.StatusInternalServerError, "StatusInternalServerError", fmt.Sprintf("deliver update to %d chats", unsend))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func writeErr(w http.ResponseWriter, status int, code, description string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(bothttp.ApiErrorResponse{
		Code:        new(code),
		Description: new(description),
	})
}
