package domain

import (
	"slices"
	"time"

	"github.com/google/uuid"
)

type LinkSubscription struct {
	userID     int64
	resID      uuid.UUID
	link       string
	tags       []string
	lastUpdate time.Time
}

func NewLinkSubscription(userID int64, resID uuid.UUID, link string, tags ...string) *LinkSubscription {
	if tags == nil {
		tags = []string{}
	}

	return &LinkSubscription{
		userID: userID,
		resID:  resID,
		link:   link,
		tags:   tags,
	}
}

func (ls *LinkSubscription) UserID() int64 {
	return ls.userID
}

func (ls *LinkSubscription) ResourceID() uuid.UUID {
	return ls.resID
}

func (ls *LinkSubscription) Link() string {
	return ls.link
}

func (ls *LinkSubscription) Tags() []string {
	return slices.Clone(ls.tags)
}

func (ls *LinkSubscription) AddTag(tag string) {
	ls.tags = append(ls.tags, tag)
}

func (ls *LinkSubscription) LastUpdate() time.Time {
	return ls.lastUpdate
}

func (ls *LinkSubscription) SetLastUpdate(updatedAt time.Time) {
	ls.lastUpdate = updatedAt
}
