package app

import (
	"strings"
	"testing"
)

func TestErrorContainsOnlySafeMessage(t *testing.T) {
	secret := "SECRET_CANARY_123"
	err := (&Error{Outcome: OutcomeConfiguration, Op: "validate", Message: "invalid value"}).Error()
	if strings.Contains(err, secret) {
		t.Fatal("secret leaked")
	}
}

func TestStableExitCodes(t *testing.T) {
	tests := map[Outcome]int{OutcomeSuccess: 0, OutcomeCancelled: 2, OutcomeConfiguration: 3, OutcomeCompatibility: 4, OutcomeSecurity: 5, OutcomeOperation: 1}
	for outcome, want := range tests {
		if got := outcome.ExitCode(); got != want {
			t.Fatalf("outcome %d code=%d want=%d", outcome, got, want)
		}
	}
}
