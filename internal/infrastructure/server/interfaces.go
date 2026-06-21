package server

import "context"

type Runner interface {
	Start(context.Context) error
	Stop(context.Context) error
}
