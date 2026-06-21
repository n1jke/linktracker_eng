package server

import (
	"context"
	"errors"
)

type MultiServer struct {
	servers []Runner
}

func NewMultiServer(servers ...Runner) *MultiServer {
	return &MultiServer{
		servers: servers,
	}
}

func (m *MultiServer) Start(ctx context.Context) error {
	stopCh := make(chan error, 1)

	for i := range m.servers {
		go func() {
			err := m.servers[i].Start(ctx)
			if err != nil {
				select {
				case stopCh <- err:
				default:
				}
			}
		}()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-stopCh:
		return err
	}
}

func (m *MultiServer) Stop(ctx context.Context) error {
	var err error

	for i := range m.servers {
		if errIn := m.servers[i].Stop(ctx); errIn != nil {
			err = errors.Join(err, errIn)
		}
	}

	return err
}
