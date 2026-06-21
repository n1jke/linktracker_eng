package application

import "errors"

var (
	// infra errors.
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrBadRequest    = errors.New("bad request")
	// application errors.
	ErrChatNotFound         = errors.New("chat not found")
	ErrLinkNotFound         = errors.New("link not found")
	ErrLinkAlreadyTracked   = errors.New("link already tracked")
	ErrSubscriptionNotFound = errors.New("subscription not found")
)
