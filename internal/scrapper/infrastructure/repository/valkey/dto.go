package cache

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/n1jke/linktracker/internal/scrapper/domain"
)

type linkDTO struct {
	UserID     int64     `json:"u_id"`
	ResID      uuid.UUID `json:"r_id"`
	Link       string    `json:"l"`
	Tags       []string  `json:"t"`
	LastUpdate time.Time `json:"lu"`
}

type JSONCodec struct{}

func (c *JSONCodec) Encode(links []*domain.LinkSubscription) (string, error) {
	dtos := make([]*linkDTO, len(links))
	for i, l := range links {
		dtos[i] = &linkDTO{
			UserID:     l.UserID(),
			ResID:      l.ResourceID(),
			Link:       l.Link(),
			Tags:       l.Tags(),
			LastUpdate: l.LastUpdate(),
		}
	}

	data, err := json.Marshal(dtos)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (c *JSONCodec) Decode(data string) ([]*domain.LinkSubscription, error) {
	var dtos []*linkDTO
	if err := json.Unmarshal([]byte(data), &dtos); err != nil {
		return nil, err
	}

	links := make([]*domain.LinkSubscription, len(dtos))
	for i := range dtos {
		links[i] = domain.NewLinkSubscription(dtos[i].UserID, dtos[i].ResID, dtos[i].Link, dtos[i].Tags...)
		links[i].SetLastUpdate(dtos[i].LastUpdate)
	}

	return links, nil
}
