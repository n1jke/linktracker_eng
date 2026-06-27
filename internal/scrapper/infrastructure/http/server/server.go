package server

import (
	"encoding/json"
	"errors"
	"net/http"

	scrapperhttp "github.com/n1jke/linktracker_eng/internal/infrastructure/transport/http/scrapper"
	"github.com/n1jke/linktracker_eng/internal/scrapper/application"
)

type ScrapperServer struct {
	useCase application.LinkService
}

func (s ScrapperServer) DeleteLinks(w http.ResponseWriter, r *http.Request, params scrapperhttp.DeleteLinksParams) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req scrapperhttp.RemoveLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body")
		return
	}

	if req.Link == nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body: missing link")
		return
	}

	err := s.useCase.UnSubscribe(r.Context(), params.TgChatId, *req.Link)
	if err != nil {
		if !errors.Is(err, application.ErrLinkNotFound) && !errors.Is(err, application.ErrChatNotFound) {
			writeErr(w, http.StatusInternalServerError, "StatusInternalServerError", "remove subscription")
			return
		}

		writeErr(w, http.StatusNotFound, "StatusNotFound", "link not found or not subscribed")

		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scrapperhttp.LinkResponse{
		Url: req.Link,
	})
}

func (s ScrapperServer) GetLinks(w http.ResponseWriter, r *http.Request, params scrapperhttp.GetLinksParams) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	links, err := s.useCase.GetLinks(r.Context(), params.TgChatId)
	if err != nil {
		if errors.Is(err, application.ErrChatNotFound) {
			writeErr(w, http.StatusNotFound, "StatusNotFound", "chat not found or not subscribed")
			return
		}

		writeErr(w, http.StatusBadRequest, "StatusBadRequest", err.Error())

		return
	}

	list := make([]scrapperhttp.LinkResponse, len(links))
	for i := range links {
		list[i] = scrapperhttp.LinkResponse{
			Id:   new(links[i].ResourceID()),
			Url:  new(links[i].Link()),
			Tags: new(links[i].Tags()),
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scrapperhttp.ListLinksResponse{
		Links: &list,
		Size:  new(len(links)),
	})
}

func (s ScrapperServer) PostLinks(w http.ResponseWriter, r *http.Request, params scrapperhttp.PostLinksParams) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req scrapperhttp.AddLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body")
		return
	}

	if req.Link == nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body: missing link")
		return
	}

	var tags []string
	if req.Tags != nil {
		tags = append(tags, *req.Tags...)
	}

	err := s.useCase.Subscribe(r.Context(), *req.Link, params.TgChatId, tags...)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrLinkAlreadyTracked):
			writeErr(w, http.StatusConflict, "StatusConflict", "link already tracked")
			return
		case errors.Is(err, application.ErrChatNotFound):
			writeErr(w, http.StatusNotFound, "StatusNotFound", "chat not found")
			return
		default:
			writeErr(w, http.StatusBadRequest, "StatusBadRequest", "subscribe to link")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scrapperhttp.LinkResponse{
		Url:  req.Link,
		Tags: req.Tags,
	})
}

func (s ScrapperServer) PostTags(w http.ResponseWriter, r *http.Request, params scrapperhttp.PostTagsParams) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req scrapperhttp.AddTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body")
		return
	}

	if req.Link == nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body: missing link")
		return
	}

	var tags []string
	if req.Tags != nil {
		tags = append(tags, *req.Tags...)
	}

	err := s.useCase.AddTags(r.Context(), params.TgChatId, *req.Link, tags)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrChatNotFound), errors.Is(err, application.ErrLinkNotFound):
			writeErr(w, http.StatusNotFound, "StatusNotFound", "chat or link not found")
			return
		case errors.Is(err, application.ErrBadRequest):
			writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid tags payload")
			return
		default:
			writeErr(w, http.StatusInternalServerError, "StatusInternalServerError", "add tags")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scrapperhttp.LinkResponse{
		Url:  req.Link,
		Tags: req.Tags,
	})
}

func (s ScrapperServer) DeleteTags(w http.ResponseWriter, r *http.Request, params scrapperhttp.DeleteTagsParams) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req scrapperhttp.ClearTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body")
		return
	}

	if req.Link == nil {
		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "invalid request body: missing link")
		return
	}

	err := s.useCase.ClearTags(r.Context(), params.TgChatId, *req.Link)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrChatNotFound), errors.Is(err, application.ErrLinkNotFound):
			writeErr(w, http.StatusNotFound, "StatusNotFound", "chat or link not found")
			return
		default:
			writeErr(w, http.StatusInternalServerError, "StatusInternalServerError", "clear tags")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(scrapperhttp.LinkResponse{
		Url:  req.Link,
		Tags: &[]string{},
	})
}

//nolint:revive // codegen method name
func (s ScrapperServer) DeleteTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	err := s.useCase.RemoveClient(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if !errors.Is(err, application.ErrChatNotFound) {
			writeErr(w, http.StatusBadRequest, "StatusBadRequest", "remove chat")
			return
		}

		writeErr(w, http.StatusNotFound, "StatusNotFound", "chat not found")

		return
	}

	w.WriteHeader(http.StatusOK)
}

//nolint:revive // codegen method name
func (s ScrapperServer) PostTgChatId(w http.ResponseWriter, r *http.Request, id int64) {
	err := s.useCase.AddClient(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if errors.Is(err, application.ErrAlreadyExists) {
			writeErr(w, http.StatusConflict, "StatusConflict", "chat already exists")
			return
		}

		writeErr(w, http.StatusBadRequest, "StatusBadRequest", "add chat")

		return
	}

	w.WriteHeader(http.StatusOK)
}

func NewScrapperServer(useCase application.LinkService) *ScrapperServer {
	return &ScrapperServer{
		useCase: useCase,
	}
}

func writeErr(w http.ResponseWriter, status int, code, description string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(scrapperhttp.ApiErrorResponse{
		Code:        new(code),
		Description: new(description),
	})
}
