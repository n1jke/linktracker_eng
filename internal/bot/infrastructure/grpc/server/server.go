package server

import (
	"context"
	"errors"

	"github.com/go-telegram/bot"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	transportgrpc "github.com/n1jke/linktracker_eng/internal/infrastructure/transport/grpc"
)

type BotGRPCServer struct {
	transportgrpc.UnimplementedBotServiceServer
	bot *bot.Bot
}

func NewBotGRPCServer(b *bot.Bot) *BotGRPCServer {
	return &BotGRPCServer{bot: b}
}

func (s *BotGRPCServer) PostUpdate(ctx context.Context, req *transportgrpc.LinkUpdate) (*emptypb.Empty, error) {
	if req.GetDescription() == "" || req.GetUrl() == "" || len(req.GetChatIds()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}

	unsent := 0

	for _, chatID := range req.GetChatIds() {
		_, err := s.bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: req.GetDescription()})
		if err != nil {
			unsent++
		}
	}

	if unsent > 0 {
		return nil, status.Error(codes.Internal, errors.New("deliver update").Error())
	}

	return &emptypb.Empty{}, nil
}
