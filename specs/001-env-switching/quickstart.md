# Quickstart Validation Guide

These scenarios become runnable after implementation. Never point installation tests at a real home
or profile.

## Prerequisites

- Supported Go 1.26 patch.
- Bash and Zsh.
- Linux or macOS test host.
- Repository root as current directory.

## 1. Static and Unit Gates

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

Expected: formatting prints no Go files; all checks pass; tests never use the operator's real home.

## 2. Strict Configuration

Create a temporary home and copy the two-project example from
[settings-schema.md](contracts/settings-schema.md) to its settings path with user-only permissions.

```bash
env HOME="$TEST_HOME" go run ./cmd/env-switcher validate
```

Expected: success without secret output or function execution. Repeat with duplicate keys, unknown fields, aliases, a
second document, invalid names, oversized input, unsafe permissions, and shell syntax errors. Each
fails closed with source-located, redacted diagnostics.

## 3. Bash Current-Shell Switching

Start clean Bash with the temporary home and source only the managed wrapper. Define colliding
variable/function names, activate `dev`, then `staging`.

Expected:

- directory and effective values match selection;
- shared definitions exist unless overridden;
- quotes, spaces, dollar signs, and metacharacters remain literal;
- obsolete managed variables/functions are removed;
- overwritten pre-existing definitions are not restored after successful switching;
- unrelated names remain unchanged;
- payload and secrets are not printed.

Force activation failure. Directory, definitions, and ownership state must equal pre-attempt state.

## 4. Zsh Current-Shell Switching

Repeat scenario 3 in clean Zsh, including Zsh-specific function validation. Test function keys where
the PTY supports them and all alternatives from [cli.md](contracts/cli.md).

## 5. Payload Rejection

For each shell, test non-zero CLI status, empty payload, missing/mismatched trailer, wrong shell,
unsupported version, bytes outside envelope, excessive size, and truncated quoting.

Expected: no evaluation, unchanged current-shell state, and no payload echoed in errors.

## 6. Idempotent Installation

In temporary homes, create representative `.bashrc`/`.zshrc`, then run confirmed install twice.

Expected:

- executable exists at `~/.env-switcher/env-switcher` and is executable;
- exactly one managed block exists and second install leaves profile bytes unchanged;
- unrelated content and mode remain;
- backups/metadata are user-only;
- real profiles remain untouched.

Test malformed markers, read-only parents, symlinks, interrupted writes, and concurrent installation;
all fail according to [installation.md](contracts/installation.md).

## 7. Rollback and Uninstall

Update a block, restore its earlier verified backup, and confirm exact profile bytes/mode. Uninstall
and confirm only the managed block and approved executable are removed; settings/backups remain.

## 8. TUI Acceptance

In PTY tests validate navigation, confirmation, F2/F3/F4/F5/F10, alternatives, editor suspend/resume,
invalid reload retention, small-terminal behavior, cancellation, and redaction. Exit actions emit no
payload for unconfirmed selection.

Verify F2 warns before deliberately showing the complete unmasked file, cancellation shows no file
content, and F3 is user-directed editing. Verify first-run and changed-function warnings and prove
that F2, F3, validation, and reload execute zero configured functions. Measure SC-001 from an open
project list: selection completes within 10 seconds and three deliberate interactions.

Run exactly 100 alternating activations in Bash and exactly 100 in Zsh. After each sequence, verify
no obsolete managed variable/function remains and every unrelated definition is unchanged.

Secret-canary validation excludes the intentional confirmed F2 view and user-directed F3 editor;
logs, errors, diagnostics, crash reports, and shell-integration diagnostics must contain zero canary
values. Inspect child-process argument vectors and assert that values and function bodies never
appear there. Verify acknowledgement metadata is current-user owned, `0600`, atomically replaced,
contains only schema version plus digest, and contains no secret canary or function-body fragment.

Run the maximum-scale reload fixture (100 projects, 100 variables, and 100 functions per project)
on both Linux and macOS CI. Fail the job if the measured reload exceeds 2 seconds, and retain only
the timing/status report—not settings content—as CI evidence.

## 9. Release Matrix

Build and smoke-test:

```text
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
```

Each artifact reports version metadata, passes native validation smoke tests, and has SHA-256.
Release acceptance requires all Linux/macOS Bash/Zsh integration jobs to pass.
