package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/n1jke/linktracker_eng/internal/agent/domain"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type MetricsRecorder interface {
	Observe(scope, scopeType string, start time.Time)
}

const (
	hfTemplate = "https://router.huggingface.co/v1/chat/completions"
	scope      = "ai_agent"
)

type Summarizer struct {
	logger  *slog.Logger
	prompt  string
	model   string
	hfToken string
	client  HTTPClient
	metrics MetricsRecorder
}

func NewSummarizer(logger *slog.Logger, prompt, model, hfToken string, client HTTPClient, m MetricsRecorder) *Summarizer {
	return &Summarizer{
		logger:  logger.With(slog.String("module", "summarizer")),
		prompt:  prompt,
		model:   model,
		hfToken: hfToken,
		client:  client,
		metrics: m,
	}
}

type HgMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type HgBody struct {
	Messages []HgMsg `json:"messages"`
	Model    string  `json:"model"`
	Stream   bool    `json:"stream"`
}

type HgResp struct {
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (s *Summarizer) Handle(ctx context.Context, d domain.Description) (domain.Description, error) {
	defer s.metrics.Observe(scope, s.model, time.Now())

	jsonBody, err := json.Marshal(HgBody{
		Messages: []HgMsg{{
			Role:    "user",
			Content: s.prompt + string(d),
		}},
		Model:  s.model,
		Stream: false,
	})
	if err != nil {
		s.logger.Error("marshal req body", slog.Any("err", err))
		return domain.Description(""), fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hfTemplate, bytes.NewReader(jsonBody))
	if err != nil {
		return domain.Description(""), err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.hfToken)

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("sending request", slog.Any("err", err))
		return domain.Description(""), fmt.Errorf("sending request: %w", err)
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			s.logger.ErrorContext(ctx, "close resp body", slog.Any("err", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("hugging face unexpected", slog.Int("status code", resp.StatusCode))
		return domain.Description(""), fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var data HgResp

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		s.logger.Error("decode response", slog.Any("err", err))
		return domain.Description(""), err
	}

	if len(data.Choices) != 1 {
		return domain.Description(""), fmt.Errorf("invalid resp len from hg: %d", len(data.Choices))
	}

	return domain.Description(data.Choices[0].Message.Content), nil
}
