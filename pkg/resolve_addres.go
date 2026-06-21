package pkg

import (
	"errors"
	"net/url"
)

var ErrEmptyAddr = errors.New("empty address")

func ResolveAddr(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if u.Host != "" {
		return u.Host, nil
	}

	return "", ErrEmptyAddr
}
