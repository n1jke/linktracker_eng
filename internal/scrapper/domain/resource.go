package domain

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrInvalidResource = errors.New("provided invalid resource")
	ErrInvalidPath     = errors.New("provided invalid path")
)

type TrackedLink struct {
	id    uuid.UUID
	path  string
	rType LinkSource
}

func NewTrackedLink(p string) (*TrackedLink, error) {
	u, err := url.Parse(strings.TrimSpace(p))
	if err != nil {
		return nil, err
	}

	source, err := CheckResource(u)
	if err != nil {
		return nil, err
	}

	return &TrackedLink{
		id:    uuid.New(),
		path:  cleanURL(u, source),
		rType: source,
	}, nil
}

func CreateTrackedLink(id uuid.UUID, p string, rType LinkSource) (*TrackedLink, error) {
	if rType != StackOverflow && rType != GitHub {
		return nil, ErrInvalidResource
	}

	return &TrackedLink{
		id:    id,
		path:  p,
		rType: rType,
	}, nil
}

func (tl *TrackedLink) ID() uuid.UUID {
	return tl.id
}

func (tl *TrackedLink) Path() string {
	return tl.path
}

func (tl *TrackedLink) Type() LinkSource {
	return tl.rType
}

type LinkSource string

const (
	StackOverflow LinkSource = "stackoverflow"
	GitHub        LinkSource = "github"
)

func CheckResource(u *url.URL) (LinkSource, error) {
	if u.Hostname() == "" {
		return "", fmt.Errorf("%w: host is empty", ErrInvalidResource)
	}

	parts := splitPath(u.Path)
	switch u.Hostname() {
	case "github.com":
		if len(parts) < 2 {
			return "", fmt.Errorf("%w: expected github.com/owner/repo", ErrInvalidPath)
		}

		return GitHub, nil
	case "stackoverflow.com":
		if len(parts) < 2 || parts[0] != "questions" {
			return "", fmt.Errorf("%w: expected stackoverflow.com/questions/", ErrInvalidPath)
		}

		return StackOverflow, nil
	default:
		return "", ErrInvalidResource
	}
}

func splitPath(p string) []string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

func cleanURL(u *url.URL, source LinkSource) string {
	host := strings.ToLower(u.Hostname())
	cleanPath := path.Clean("/" + strings.TrimSpace(u.Path))

	switch source {
	case GitHub:
		parts := splitPath(cleanPath)
		if len(parts) >= 2 {
			cleanPath = "/" + parts[0] + "/" + parts[1]
		}
	case StackOverflow:
		parts := splitPath(cleanPath)
		if len(parts) >= 2 && parts[0] == "questions" {
			cleanPath = "/" + parts[0] + "/" + parts[1]
		}
	}

	return host + cleanPath
}
