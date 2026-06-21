package domain_test

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/n1jke/linktracker/internal/agent/domain"
)

func TestDescription_SplitToTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    domain.Description
		want []string
	}{
		{
			name: "askii chars",
			d:    "hello, world<  ",
			want: []string{"hello", "world"},
		},
		{
			name: "with digits",
			d:    "hello123world test",
			want: []string{"hello123world", "test"},
		},
		{
			name: "unicode",
			d:    "пупупу, !пум",
			want: []string{"пупупу", "пум"},
		},
		{
			name: "0 words",
			d:    "!@#$%",
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.d.SplitToTokens()

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLinkEvent_ID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		id             domain.ID
		link           domain.Link
		description    domain.Description
		newDescription domain.Description
		author         domain.Author
	}{
		{
			name:           "valid askii",
			id:             1,
			link:           "https://github.com/n1jke/n1jke",
			description:    "wow",
			newDescription: "can be better",
			author:         "n1jke",
		},
		{
			name:           "с кириллицей",
			id:             1,
			link:           "https://stackoverflow.com/questions/1",
			description:    "ого",
			newDescription: "пу",
			author:         "пупупу",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			l := domain.NewLinkEvent(tt.id, tt.link, tt.description, tt.author)

			assert.Equal(t, tt.id, l.ID())
			assert.Equal(t, tt.link, l.Link())
			assert.Equal(t, tt.description, l.Description())
			assert.Equal(t, utf8.RuneCountInString(string(tt.description)), l.DescriptionLen())
			assert.Equal(t, tt.author, l.Author())

			nl := l.WithSummarizedDescription(tt.newDescription)

			assert.Equal(t, tt.newDescription, nl.Description())
			assert.Equal(t, tt.id, nl.ID())
			assert.Equal(t, tt.link, nl.Link())
			assert.Equal(t, tt.author, nl.Author())
		})
	}
}
