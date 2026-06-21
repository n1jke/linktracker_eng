package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/n1jke/linktracker/internal/agent/domain"
)

func TestFilteringPolicy_CheckEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *domain.FilterConfig
		e    *domain.LinkEvent
		want domain.Decision
	}{
		{
			name: "stop word",
			cfg: &domain.FilterConfig{
				StopWords: []string{"spam"},
			},
			e:    domain.NewLinkEvent(1, "url", "this spam message", "me"),
			want: domain.NewDecision(domain.Ignore, ""),
		},
		{
			name: "stop word upper register",
			cfg: &domain.FilterConfig{
				StopWords: []string{"SPAM"},
			},
			e:    domain.NewLinkEvent(1, "url", "this spam message", "me"),
			want: domain.NewDecision(domain.Ignore, ""),
		},
		{
			name: "excluded author",
			cfg: &domain.FilterConfig{
				ExcludedAuthors: []domain.Author{"n1jke", "wow", "ddd xd(("},
			},
			e:    domain.NewLinkEvent(1, "url", "valid", "n1jke"),
			want: domain.NewDecision(domain.Ignore, ""),
		},
		{
			name: "to short",
			cfg: &domain.FilterConfig{
				MinLength: 50,
			},
			e:    domain.NewLinkEvent(1, "url", "small", "n1jke"),
			want: domain.NewDecision(domain.Ignore, ""),
		},
		{
			name: "normal",
			cfg: &domain.FilterConfig{
				MinLength: 5,
				Threshold: 100,
			},
			e:    domain.NewLinkEvent(1, "url", "ddd at the end of deadline?!", "n1jke"),
			want: domain.NewDecision(domain.Pass, domain.Medium),
		},
		{
			name: "len > threshold",
			cfg: &domain.FilterConfig{
				HighPriority: []string{"crit"},
				LowPriority:  []string{"typo"},
				MinLength:    5,
				Threshold:    10,
			},
			e:    domain.NewLinkEvent(1, "url", "crit error: ddd at the end of deadline?!", "n1jke"),
			want: domain.NewDecision(domain.Summarize, domain.High),
		},
		{
			name: "low priority",
			cfg: &domain.FilterConfig{
				HighPriority: []string{"crit"},
				LowPriority:  []string{"typo"},
				MinLength:    5,
				Threshold:    10,
			},
			e:    domain.NewLinkEvent(1, "url", "typo err: structs names", "n1jke"),
			want: domain.NewDecision(domain.Summarize, domain.Low),
		},
		{
			name: "t2",
			cfg: &domain.FilterConfig{
				HighPriority: []string{"crit"},
				LowPriority:  []string{"typo"},
				MinLength:    5,
				Threshold:    10,
			},
			e:    domain.NewLinkEvent(1, "url", "ddd at the end of deadline?!", "n1jke"),
			want: domain.NewDecision(domain.Summarize, domain.Medium),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NotNil(t, tt.cfg)

			p := domain.NewFilteringPolicy(tt.cfg)
			got := p.CheckEvent(tt.e)

			assert.Equal(t, tt.want.Action, got.Action)
			assert.Equal(t, tt.want.Priority, got.Priority)
		})
	}
}
