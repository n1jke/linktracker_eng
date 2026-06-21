package domain

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type (
	ID          int64
	Link        string
	Description string
	Author      string
)

func (d Description) SplitToTokens() []string {
	prep := strings.ToLower(string(d))

	return strings.FieldsFunc(prep, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

type LinkEvent struct {
	id          ID
	link        Link
	description Description
	author      Author
}

func NewLinkEvent(id ID, link Link, descr Description, author Author) *LinkEvent {
	return &LinkEvent{
		id:          id,
		link:        link,
		description: descr,
		author:      author,
	}
}

func (l LinkEvent) ID() ID {
	return l.id
}

func (l LinkEvent) Link() Link {
	return l.link
}

func (l LinkEvent) Description() Description {
	return l.description
}

func (l LinkEvent) DescriptionLen() int {
	return utf8.RuneCountInString(string(l.description))
}

func (l LinkEvent) WithSummarizedDescription(d Description) *LinkEvent {
	return NewLinkEvent(l.id, l.link, d, l.author)
}

func (l LinkEvent) Author() Author {
	return l.author
}
