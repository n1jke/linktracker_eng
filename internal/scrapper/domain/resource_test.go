package domain_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/n1jke/linktracker_eng/internal/scrapper/domain"
)

func TestCheckResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		inputURL string
		want     domain.LinkSource
		wantErr  bool
	}{
		{
			name:     "valid github with owner and repo",
			inputURL: "https://github.com/golang/go",
			want:     domain.GitHub,
		},
		{
			name:     "valid github with trailing slash",
			inputURL: "https://github.com/n1jke/oop-bsuir-2025/",
			want:     domain.GitHub,
		},
		{
			name:     "valid stackoverflow question with trailing slash",
			inputURL: "https://stackoverflow.com/questions/20778771/",
			want:     domain.StackOverflow,
		},
		{
			name:     "valid stackoverflow question",
			inputURL: "https://stackoverflow.com/questions/42235",
			want:     domain.StackOverflow,
		},
		{
			name:     "unsupported host",
			inputURL: "https://example.com/some/path",
			wantErr:  true,
		},
		{
			name:     "invalid github without repo",
			inputURL: "https://github.com/n1jke",
			wantErr:  true,
		},
		{
			name:     "invalid github root",
			inputURL: "https://github.com/",
			wantErr:  true,
		},
		{
			name:     "invalid stackoverflow other path",
			inputURL: "https://stackoverflow.com/help",
			wantErr:  true,
		},
		{
			name:     "invalid stackoverflow questions root",
			inputURL: "https://stackoverflow.com/questions",
			wantErr:  true,
		},
		{
			name:     "empty host",
			inputURL: "/questions/123",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			u, parseErr := url.Parse(tt.inputURL)
			require.NoError(t, parseErr, "url should parse in test case")

			got, err := domain.CheckResource(u)

			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
