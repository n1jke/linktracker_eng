package server

import (
	"context"
	"net"

	"google.golang.org/grpc"
)

type GRPCServer struct {
	server *grpc.Server
	addr   string
}

func NewGRPCServer(addr string, opt []grpc.ServerOption, register func(*grpc.Server)) *GRPCServer {
	svr := grpc.NewServer(opt...)
	register(svr)

	return &GRPCServer{
		server: svr,
		addr:   addr,
	}
}

func (r *GRPCServer) Start(_ context.Context) error {
	lis, err := net.Listen("tcp", r.addr)
	if err != nil {
		return err
	}

	return r.server.Serve(lis)
}

func (r *GRPCServer) Stop(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		r.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		r.server.Stop()
		return ctx.Err()
	}
}
