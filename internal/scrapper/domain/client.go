package domain

type Client struct {
	id int64
}

func NewClient(id int64) *Client {
	return &Client{
		id: id,
	}
}

func (c *Client) ID() int64 {
	return c.id
}
