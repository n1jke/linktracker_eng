package application

type ClientState int

const (
	Available ClientState = iota
	AwaitLink
	AwaitTags
)

type ChatState struct {
	ChatID int64
	State  ClientState
	URL    string
	Tags   []string
}
