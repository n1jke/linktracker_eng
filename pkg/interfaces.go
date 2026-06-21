package pkg

import "context"

type ServerRunner interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type ResourceCloser interface {
	Close() error
}

type SchedulerRunner interface {
	Start(context.Context) error
	Stop() error
}
