package domain

type Decision struct {
	Action   Action
	Priority Priority
}

func NewDecision(a Action, p Priority) Decision {
	return Decision{
		Action:   a,
		Priority: p,
	}
}

type Action int

const (
	Pass Action = iota
	Ignore
	Summarize
)

type Priority string

const (
	High   Priority = "HIGH"
	Medium Priority = "MEDIUM"
	Low    Priority = "LOW"
)

func (p Priority) weight() int {
	switch p {
	case High:
		return 3
	case Medium:
		return 2
	case Low:
		return 1
	default:
		return 0
	}
}

func (p Priority) IsHigherThan(other Priority) bool {
	return p.weight() > other.weight()
}
