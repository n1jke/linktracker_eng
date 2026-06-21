package application

import "errors"

var (
	ErrAlreadyRegistered = errors.New("chat already registered")
	ErrAlreadyTracked    = errors.New("link already tracked")
	ErrChatNotFound      = errors.New("chat not found")
	ErrLinkNotFound      = errors.New("link not found")
	ErrBadRequest        = errors.New("bad request")
	ErrUnavailable       = errors.New("service unavailable")
)
