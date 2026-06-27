package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

//go:generate mockgen -source=command_service.go -exclude_interfaces=MetricsRecorder -destination=mocks/mocks.go -package=mocks

type ChatStateRepository interface {
	Get(chatID int64) ChatState
	StartTracking(chatID int64)
	SetURL(chatID int64, url string)
	SetTags(chatID int64, tags []string)
	Remove(chatID int64)
}

type Scrapper interface {
	RegisterChat(ctx context.Context, chatID int64) error
	TrackLink(ctx context.Context, chatID int64, link string, tags []string) error
	AddTags(ctx context.Context, chatID int64, link string, tags []string) error
	ClearTags(ctx context.Context, chatID int64, link string) error
	UntrackLink(ctx context.Context, chatID int64, link string) error
	ListLinks(ctx context.Context, chatID int64) ([]*domain.LinkSubscription, error)
}

type MetricsRecorder interface {
	AddCommandRate(cmd string)
}

const (
	CommandStart   = "start"
	CommandHelp    = "help"
	CommandTrack   = "track"
	CommandUntrack = "untrack"
	CommandList    = "list"
	CommandAddTags = "addtags"
	CommandDelTags = "cleartags"
	CommandCancel  = "cancel"
	CommandURL     = "url"
	CommandTags    = "tags"

	StartMsg          = "Добро пожаловать! \nИспользуйте /help, чтобы посмотреть доступные команды."
	DefaultMsg        = "Неизвестная команда. \nВоспользуйтесь /help, чтобы посмотреть список доступных команд"
	TrackStartMsg     = "Пришлите ссылку для отслеживания: "
	TrackAwaitTagsMsg = "Пришлите теги командой: /tags <tag1> <tag2> ..."
	TrackCancelMsg    = "Отслеживание отменено"
	TrackDoneMsg      = "Ссылка отслеживается"
	TrackExistsMsg    = "Ссылка уже отслеживается"
	UntrackUsageMsg   = "Формат: /untrack <url>"
	UntrackDoneMsg    = "Ссылка больше не отслеживается ):"
	UntrackNotFound   = "Ссылка не отслеживается"
	AddTagsUsageMsg   = "Формат: /addtags <url> <tag1> <tag2> ..."
	AddTagsDoneMsg    = "Теги добавлены"
	ClearTagsUsageMsg = "Формат: /cleartags <url>"
	ClearTagsDoneMsg  = "Теги очищены"
	ListEmptyMsg      = "Нет отслеживаемых ссылок ):"
	StartExistsMsg    = "Чат уже зарегистрирован"
	ChatNotFoundMsg   = "Чат не зарегистрирован. Используйте /start"
	BadRequestMsg     = "Некорректный запрос"
	UnavailableMsg    = "Сервис временно недоступен, попробуйте позже"

	ServerErrorMsg = "ошибка на стороне сервера );"
)

type CommandInfo struct {
	Name        string
	Description string
}

type CommandUseCase struct {
	states   ChatStateRepository
	scrapper Scrapper
	metrics  MetricsRecorder
}

func NewCommandUseCase(repository ChatStateRepository, scrapper Scrapper, m MetricsRecorder) *CommandUseCase {
	return &CommandUseCase{
		states:   repository,
		scrapper: scrapper,
		metrics:  m,
	}
}

func (uc *CommandUseCase) Handle(ctx context.Context, text string, chatID int64) (string, error) {
	session := uc.states.Get(chatID)
	cmd, args := parseInput(text)
	uc.metrics.AddCommandRate(cmd)

	switch session.State {
	case AwaitLink:
		return uc.awaitLinkHandle(chatID, cmd, args)
	case AwaitTags:
		return uc.awaitTagsHandle(ctx, chatID, cmd, args)
	case Available:
		return uc.availableChatHandle(ctx, chatID, cmd, args)
	default:
		return "", nil
	}
}

func (uc *CommandUseCase) SupportedCommands() []CommandInfo {
	commands := []CommandInfo{
		{Name: CommandStart, Description: "регистрация пользователя и старт бота"},
		{Name: CommandHelp, Description: "список доступных команд"},
		{Name: CommandTrack, Description: "начать отслеживание ссылки"},
		{Name: CommandUntrack, Description: "перестать отслеживать ссылку"},
		{Name: CommandAddTags, Description: "добавить теги к отслеживаемой ссылке"},
		{Name: CommandDelTags, Description: "очистить теги отслеживаемой ссылки"},
		{Name: CommandList, Description: "список ссылок"},
		{Name: CommandURL, Description: "формирование отслеживаемой ссылки"},
		{Name: CommandTags, Description: "формирование тегов для ссылки"},
	}

	return commands
}

func (uc *CommandUseCase) availableChatHandle(ctx context.Context, chatID int64, cmd string, args []string) (string, error) {
	switch cmd {
	case CommandStart:
		uc.states.Remove(chatID)

		err := uc.scrapper.RegisterChat(ctx, chatID)
		if err != nil {
			return mapError(err)
		}

		return StartMsg, nil

	case CommandHelp:
		return renderHelp(uc.SupportedCommands()), nil

	case CommandTrack:
		uc.states.StartTracking(chatID)
		return TrackStartMsg, nil

	case CommandUntrack:
		if len(args) != 1 {
			return UntrackUsageMsg, nil
		}

		if err := uc.scrapper.UntrackLink(ctx, chatID, args[0]); err != nil {
			return mapError(err)
		}

		return UntrackDoneMsg, nil

	case CommandList:
		links, err := uc.scrapper.ListLinks(ctx, chatID)
		if err != nil {
			return mapError(err)
		}

		if len(links) == 0 {
			return ListEmptyMsg, nil
		}

		return renderLinks(links), nil
	case CommandAddTags:
		if len(args) < 2 {
			return AddTagsUsageMsg, nil
		}

		if err := uc.scrapper.AddTags(ctx, chatID, args[0], args[1:]); err != nil {
			return mapError(err)
		}

		return AddTagsDoneMsg, nil
	case CommandDelTags:
		if len(args) != 1 {
			return ClearTagsUsageMsg, nil
		}

		if err := uc.scrapper.ClearTags(ctx, chatID, args[0]); err != nil {
			return mapError(err)
		}

		return ClearTagsDoneMsg, nil

	default:
		return DefaultMsg, nil
	}
}

func (uc *CommandUseCase) awaitLinkHandle(chatID int64, cmd string, args []string) (string, error) {
	switch cmd {
	case CommandURL:
		if len(args) != 1 {
			return renderHelp(uc.SupportedCommands()), nil
		}

		uc.states.SetURL(chatID, args[0])

		return TrackAwaitTagsMsg, nil
	case CommandCancel:
		uc.states.Remove(chatID)
		return TrackCancelMsg, nil

	default:
		uc.states.Remove(chatID)
		return TrackCancelMsg, nil
	}
}

func (uc *CommandUseCase) awaitTagsHandle(ctx context.Context, chatID int64, cmd string, args []string) (string, error) {
	switch cmd {
	case CommandTags:
		uc.states.SetTags(chatID, args)
		session := uc.states.Get(chatID)

		err := uc.scrapper.TrackLink(ctx, chatID, session.URL, session.Tags)
		if err != nil {
			uc.states.Remove(chatID)
			return mapError(err)
		}

		uc.states.Remove(chatID)

		return TrackDoneMsg, nil
	case CommandCancel:
		uc.states.Remove(chatID)
		return TrackCancelMsg, nil

	default:
		uc.states.Remove(chatID)
		return TrackCancelMsg, nil
	}
}

func parseInput(msg string) (string, []string) {
	parts := strings.Fields(msg)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := strings.TrimPrefix(parts[0], "/")
	if cmd == "" {
		return "", nil
	}

	if len(parts) == 1 {
		return strings.ToLower(cmd), nil
	}

	return strings.ToLower(cmd), parts[1:]
}

func renderLinks(links []*domain.LinkSubscription) string {
	lines := make([]string, 0, len(links)+1)
	lines = append(lines, "Отслеживаемые ссылки:")

	for i := range links {
		tags := links[i].Tags()

		tagPart := "без тегов"
		if len(tags) > 0 {
			tagPart = strings.Join(tags, ", ")
		}

		lines = append(lines, fmt.Sprintf("%d. %s [tags: %s]", i+1, links[i].Link(), tagPart))
	}

	return strings.Join(lines, "\n")
}

func renderHelp(commands []CommandInfo) string {
	var b strings.Builder
	b.WriteString("Доступные команды: \n")

	for i := range commands {
		b.WriteString("/")
		b.WriteString(commands[i].Name)
		b.WriteString(" -> ")
		b.WriteString(commands[i].Description)
		b.WriteByte('\n')
	}

	return b.String()
}

func mapError(err error) (string, error) {
	switch {
	case errors.Is(err, ErrAlreadyTracked):
		return TrackExistsMsg, nil
	case errors.Is(err, ErrAlreadyRegistered):
		return StartExistsMsg, nil
	case errors.Is(err, ErrLinkNotFound):
		return UntrackNotFound, nil
	case errors.Is(err, ErrChatNotFound):
		return ChatNotFoundMsg, nil
	case errors.Is(err, ErrBadRequest):
		return BadRequestMsg, nil
	case errors.Is(err, ErrUnavailable):
		return UnavailableMsg, err
	default:
		return ServerErrorMsg, err
	}
}
