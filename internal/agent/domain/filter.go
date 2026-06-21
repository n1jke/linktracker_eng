package domain

import (
	"strings"
)

type FilterConfig struct {
	StopWords       []string
	ExcludedAuthors []Author
	LowPriority     []string
	HighPriority    []string
	MinLength       int
	Threshold       int
}

type FilteringPolicy struct {
	stopWords       map[string]struct{}
	excludedAuthors map[Author]struct{}
	wordsPriority   map[string]Priority
	minLength       int
	threshold       int
}

func NewFilteringPolicy(cfg *FilterConfig) *FilteringPolicy {
	fp := &FilteringPolicy{
		stopWords:       make(map[string]struct{}, len(cfg.StopWords)),
		excludedAuthors: make(map[Author]struct{}, len(cfg.ExcludedAuthors)),
		wordsPriority:   make(map[string]Priority, len(cfg.HighPriority)+len(cfg.LowPriority)),
		minLength:       cfg.MinLength,
		threshold:       cfg.Threshold,
	}

	for i := range cfg.StopWords {
		fp.stopWords[strings.ToLower(cfg.StopWords[i])] = struct{}{}
	}

	for i := range cfg.ExcludedAuthors {
		fp.excludedAuthors[cfg.ExcludedAuthors[i]] = struct{}{}
	}

	for i := range cfg.LowPriority {
		fp.wordsPriority[strings.ToLower(cfg.LowPriority[i])] = Low
	}

	for i := range cfg.HighPriority {
		fp.wordsPriority[strings.ToLower(cfg.HighPriority[i])] = High
	}

	return fp
}

func (l *FilteringPolicy) CheckEvent(e *LinkEvent) Decision {
	if l.containsStopWord(e) || l.isAuthorExcluded(e) || e.DescriptionLen() < l.minLength {
		return NewDecision(Ignore, "")
	}

	a := Pass
	if e.DescriptionLen() >= l.threshold {
		a = Summarize
	}

	p := l.findPriority(e)

	return NewDecision(a, p)
}

func (l *FilteringPolicy) containsStopWord(e *LinkEvent) bool {
	words := e.description.SplitToTokens()

	for i := range words {
		if _, find := l.stopWords[words[i]]; find {
			return true
		}
	}

	return false
}

func (l *FilteringPolicy) isAuthorExcluded(e *LinkEvent) bool {
	_, match := l.excludedAuthors[e.Author()]

	return match
}

func (l *FilteringPolicy) findPriority(e *LinkEvent) Priority {
	words := e.description.SplitToTokens()
	priority := Medium

	for i := range words {
		p, ok := l.wordsPriority[words[i]]
		if p == High {
			return High
		}

		if ok {
			priority = p
		}
	}

	return priority
}
