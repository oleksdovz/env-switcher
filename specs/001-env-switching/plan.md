# Implementation Plan: Terminal Environment Switching

**Branch**: `001-env-switching` | **Date**: 2026-08-23 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/001-env-switching/spec.md`

## Summary

Build a single-user Go CLI with a Bubble Tea TUI that strictly validates
`~/.env-switcher/settings.yaml`, lets a user select a project, and emits a versioned Bash- or
Zsh-specific activation payload. A minimal installed wrapper captures payload output separately
from diagnostics and evaluates it in the current shell only after successful validation. The
payload transactionally updates the working directory, managed variables, managed functions, and
session-local ownership metadata, rolling back the attempted activation on failure.

The installer copies the executable to `~/.env-switcher/env-switcher` and atomically manages one
marked block in `~/.bashrc` or `~/.zshrc`, with timestamped backups, explicit confirmation,
idempotent updates, and rollback. Configuration, payload generation, wrapper behavior, installation,
and rollback receive unit, contract, and real-shell integration coverage on Linux and macOS.
F2 intentionally reveals the complete local settings file only after a sensitive-data warning;
all non-interactive channels remain redacted. Configured functions are trusted user code and are
never executed by view, edit, validation, or reload operations.

## Technical Context

**Language/Version**: Go 1.26.x; CI pins the latest supported 1.26 patch and release builds use the
same declared toolchain. Reassess Go 1.27 after its patch line has matured.

**Primary Dependencies**: Bubble Tea v2; Bubbles v2 list/help components where useful; Lip Gloss v2;
`gopkg.in/yaml.v3` for stable YAML node decoding. All versions are pinned in `go.mod`/`go.sum`;
standard library is preferred for filesystem, quoting, installation, and process execution.

**Storage**: Local files only: `~/.env-switcher/settings.yaml`,
`~/.env-switcher/env-switcher`, user-only trusted-function acknowledgement metadata, and timestamped
backups under `~/.env-switcher/backups/`. Managed variable/function ownership is session-local shell
state; acknowledgement metadata contains only a digest and never function bodies or secrets.

**Testing**: Go `testing`, table tests, fuzz tests for YAML and quoting, golden Bash/Zsh payload and
managed-block fixtures, real `bash`/`zsh` subprocess integration tests, isolated temporary homes,
race detector, `go vet`, and Linux/macOS CI. Security tests inspect stdout, stderr, logs, crash
artifacts, child-process arguments, profiles, backups, and acknowledgement metadata with unique
secret canaries.

**Target Platform**: Linux and macOS; interactive Bash and Zsh. Initial releases target
`linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`.

**Project Type**: Single command-line application with an interactive TUI and shell integration.

**Performance Goals**: Validate 100 projects with 100 variables and 100 functions each within
2 seconds in CI; typical TUI input-to-render latency below 100 ms; activation preparation below
500 ms excluding user input. The Linux and macOS CI jobs fail when the 2-second validation/reload
threshold is exceeded; timing evidence is retained as a test artifact.

**Constraints**: Offline; no daemon, database, network, telemetry, or privileged install; one
self-contained executable; user-only configuration; stdout payload contains no diagnostics;
secrets never enter argv or logs; profile edits are atomic and recoverable.

**Scale/Scope**: One local user and one active shell session per invocation; up to 100 projects,
100 variables and 100 functions per project, and 1 MiB maximum settings file. Bash/Zsh only;
remote configuration and secret-manager integration are out of scope.

## Constitution Check

*GATE: Passed before Phase 0 research and re-checked after Phase 1 design.*

| Constitutional gate | Design evidence | Status |
|---------------------|-----------------|--------|
| Portable self-contained Go CLI for Linux/macOS | Single Go module; four release targets; no daemon | PASS |
| Bash/Zsh current-shell integration | Minimal wrapper plus versioned shell-specific payload | PASS |
| Explicit, idempotent, reversible installation | Managed block, atomic replacement, backup and rollback | PASS |
| Strict settings source of truth | One YAML file; schema, duplicate, unknown-key validation | PASS |
| No partial environment application | Prevalidated payload with in-shell snapshot and rollback | PASS |
| Remove obsolete managed names only | Session-local ownership metadata controls cleanup | PASS |
| Secrets remain local and protected | Constitution v2.0.0 permits confirmed F2 and user-directed F3; all other channels are redacted | PASS |
| Trusted-function acknowledgement is private and minimal | Atomic `0600` digest-only metadata; no bodies or values | PASS |
| Stable TUI keys and fallbacks | F2-F5/F10 plus alternative bindings in CLI contract | PASS |
| Required quality gates | Unit, fuzz, golden, real-shell, Linux/macOS, vet/race tests | PASS |
| Dependency justification and pinning | Minimal dependency set; exact versions in module files | PASS |

Post-design re-check: constitution v2.0.0 explicitly distinguishes intentional local F2/F3
disclosure from prohibited logs, errors, diagnostics, crash reports, and user-visible shell
integration output. All gates pass.

## Project Structure

### Documentation (this feature)

```text
specs/001-env-switching/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── cli.md
│   ├── installation.md
│   ├── settings-schema.md
│   └── shell-payload.md
├── checklists/
│   └── requirements.md
└── tasks.md              # Dependency-ordered implementation work
```

### Source Code (repository root)

```text
cmd/
└── env-switcher/
    └── main.go

internal/
├── app/                    # Orchestration and exit-status mapping
├── config/                 # YAML nodes, strict decode, validation, defaults
├── environment/            # Merge and activation transaction model
├── shell/
│   ├── contract.go         # Envelope and shell-neutral operations
│   ├── quote.go            # Literal-safe Bash/Zsh quoting
│   ├── bash.go
│   └── zsh.go
├── install/                # Profile discovery, managed blocks, backup, rollback
├── editor/                 # Default-editor resolution and controlled execution
└── tui/                    # Bubble Tea model, messages, views, key map

testdata/
├── config/                 # Valid/invalid/redaction YAML fixtures
├── payload/                # Bash/Zsh golden outputs
└── profiles/               # Managed-block and rollback fixtures

tests/
├── contract/                # Contract and golden compatibility tests
├── integration/             # Real-shell and isolated-home tests
└── release/                 # Built-artifact smoke and checksum tests
```

**Structure Decision**: Use one Go module and keep executable entry code minimal. Reusable logic
stays in `internal/` so configuration, activation, installation, and TUI behavior can be tested
without the complete interactive program. Shell-neutral operations are separated from Bash/Zsh
rendering so compatibility differences remain explicit.

## Delivery Strategy

1. Establish module/dependency pins, command boundary, error taxonomy, and fixtures.
2. Implement strict settings decode and semantic validation before TUI or shell output.
3. Implement deterministic merge and managed-name lifecycle as shell-neutral data.
4. Implement Bash/Zsh quoting and payload renderers with golden and fuzz tests.
5. Implement minimal wrappers and real-shell activation/rollback integration tests.
6. Implement atomic installation, backups, managed-block reconciliation, rollback, and uninstall.
7. Add the Bubble Tea TUI and editor/view/reload/install actions over tested application services.
8. Add cross-platform releases, checksums, smoke tests, security review, and documentation.

Validation evidence is produced under `tests/contract/`, `tests/integration/`, and `tests/release/`;
CI publishes test reports, the bounded-scale timing result, and release checksum verification. No
evidence artifact may contain configuration values, function bodies, or captured activation payloads.

Every stage keeps formatting, `go test ./...`, `go vet ./...`, and applicable contract tests green.
No profile mutation test may use a real user home directory.

## Complexity Tracking

No constitutional violations require justification. Separate Bash/Zsh renderers and a transactional
payload are necessary because current-shell mutation, quoting, function lifecycle, and rollback
cannot be delegated to the child process.
