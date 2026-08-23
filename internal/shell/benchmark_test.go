package shell

import (
	"fmt"
	"testing"
	"time"

	"github.com/dolf/env-switcher/internal/environment"
)

func benchmarkEnvironment() *environment.Effective {
	e := &environment.Effective{Project: "benchmark", Shell: "bash"}
	for i := 0; i < 100; i++ {
		e.Variables = append(e.Variables, environment.Variable{Name: fmt.Sprintf("VAR_%03d", i), Value: "literal value"})
		e.Functions = append(e.Functions, environment.Function{Name: fmt.Sprintf("fn_%03d", i), Body: ":"})
	}
	return e
}
func BenchmarkPayloadPreparation(b *testing.B) {
	e := benchmarkEnvironment()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Render(e); err != nil {
			b.Fatal(err)
		}
	}
}
func TestTypicalPayloadPreparationUnder500ms(t *testing.T) {
	started := time.Now()
	if _, err := Render(benchmarkEnvironment()); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("payload preparation took %s", elapsed)
	}
}
