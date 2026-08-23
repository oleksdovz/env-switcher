package integration

import (
	"strings"
	"testing"

	"github.com/dolf/env-switcher/internal/config"
)

func TestInvalidSettingsErrorRedactsScalarValue(t *testing.T) {
	const canary = "SECRET_CANARY_DO_NOT_PRINT"
	input := "version: 1\nenvs:\n  dev:\n    project: /tmp\n    env-vars:\n      KEY:\n        nested: " + canary + "\n"
	_, err := config.Parse(strings.NewReader(input))
	if err == nil {
		t.Fatal("invalid scalar accepted")
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatal("secret canary leaked in validation error")
	}
}
