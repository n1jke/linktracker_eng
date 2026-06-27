package server

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	transportgrpc "github.com/n1jke/linktracker_eng/internal/infrastructure/transport/grpc"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
)

type ScrapperGRPCServer struct {
	transportgrpc.UnimplementedScrapperServiceServer
	service application.LinkService
}

func NewScrapperGRPCServer(service application.LinkService) *ScrapperGRPCServer {
	return &ScrapperGRPCServer{service: service}
}

func (s *ScrapperGRPCServer) RegisterChat(ctx context.Context, req *transportgrpc.RegisterChatRequest) (*emptypb.Empty, error) {
	if err := s.service.AddClient(ctx, req.GetChatId()); err != nil {
		return nil, mapErrors(err)
	}

	return &emptypb.Empty{}, nil
}

func (s *ScrapperGRPCServer) TrackLink(ctx context.Context, req *transportgrpc.TrackLinkRequest) (*transportgrpc.LinkResponse, error) {
	if err := s.service.Subscribe(ctx, req.GetUrl(), req.GetChatId(), req.GetTags()...); err != nil {
		return nil, mapErrors(err)
	}

	return &transportgrpc.LinkResponse{Id: "", Url: req.GetUrl(), Tags: req.GetTags()}, nil
}

func (s *ScrapperGRPCServer) UntrackLink(ctx context.Context, req *transportgrpc.UntrackLinkRequest) (*transportgrpc.LinkResponse, error) {
	if err := s.service.UnSubscribe(ctx, req.GetChatId(), req.GetUrl()); err != nil {
		return nil, mapErrors(err)
	}

	return &transportgrpc.LinkResponse{Id: "", Url: req.GetUrl()}, nil
}

func (s *ScrapperGRPCServer) ListLinks(ctx context.Context, req *transportgrpc.ListLinksRequest) (*transportgrpc.ListLinksResponse, error) {
	links, err := s.service.GetLinks(ctx, req.GetChatId())
	if err != nil {
		return nil, mapErrors(err)
	}

	resp := &transportgrpc.ListLinksResponse{}
	for _, link := range links {
		resp.Links = append(resp.Links, &transportgrpc.LinkResponse{
			Id:   link.ResourceID().String(),
			Url:  link.Link(),
			Tags: link.Tags(),
		})
	}

	resp.Size = int64(len(resp.Links))

	return resp, nil
}

func (s *ScrapperGRPCServer) AddTags(ctx context.Context, req *transportgrpc.AddTagsRequest) (*transportgrpc.LinkResponse, error) {
	if err := s.service.AddTags(ctx, req.GetChatId(), req.GetUrl(), req.GetTags()); err != nil {
		return nil, mapErrors(err)
	}

	return &transportgrpc.LinkResponse{Url: req.GetUrl(), Tags: req.GetTags()}, nil
}

func (s *ScrapperGRPCServer) ClearTags(ctx context.Context, req *transportgrpc.ClearTagsRequest) (*transportgrpc.LinkResponse, error) {
	if err := s.service.ClearTags(ctx, req.GetChatId(), req.GetUrl()); err != nil {
		return nil, mapErrors(err)
	}

	return &transportgrpc.LinkResponse{Url: req.GetUrl()}, nil
}

func mapErrors(err error) error {
	switch {
	case errors.Is(err, application.ErrAlreadyExists), errors.Is(err, application.ErrLinkAlreadyTracked):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, application.ErrBadRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, application.ErrNotFound), errors.Is(err, application.ErrChatNotFound), errors.Is(err, application.ErrLinkNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
