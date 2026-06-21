package server

import (
	"context"
	"errors"
	"net/http"
	"time"
)

type HTTPServer struct {
	server *http.Server
}

func NewHTTPServer(addr string, handler http.Handler, readTimeout, writeTimeout, idleTimeout time.Duration) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout:  idleTimeout,
		},
	}
}

func (r *HTTPServer) Start(_ context.Context) error {
	err := r.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (r *HTTPServer) Stop(ctx context.Context) error {
	return r.server.Shutdown(ctx)
}
