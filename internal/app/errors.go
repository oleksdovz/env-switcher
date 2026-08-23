package app

import "fmt"

type Outcome int

const (
	OutcomeSuccess Outcome = iota
	OutcomeCancelled
	OutcomeConfiguration
	OutcomeCompatibility
	OutcomeSecurity
	OutcomeOperation
)

func (o Outcome) ExitCode() int {
	switch o {
	case OutcomeSuccess:
		return 0
	case OutcomeCancelled:
		return 2
	case OutcomeConfiguration:
		return 3
	case OutcomeCompatibility:
		return 4
	case OutcomeSecurity:
		return 5
	default:
		return 1
	}
}

type Error struct {
	Outcome Outcome
	Op      string
	Message string
}

func (e *Error) Error() string {
	if e.Op == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}
