<!--
Sync Impact Report
- Version change: 1.0.0 -> 2.0.0
- Modified principles:
  - III. Configuration Is the Source of Truth -> trusted shell-function lifecycle added
  - IV. Secrets Stay Local and Protected -> intentional local F2/F3 disclosure exception added
  - V. Tested and Predictable Terminal UX -> F2 warning and trusted-code warnings added
- Added sections: none
- Removed sections: none
- Follow-up TODOs: none
-->
# env-switcher Constitution

## Core Principles

### I. Portable Go CLI
env-switcher MUST be implemented in Go and distributed as a self-contained executable for
supported Linux and macOS architectures. Core behavior MUST NOT depend on a graphical desktop,
platform-specific package manager, or an always-running service. Platform-specific code MUST be
isolated behind tested interfaces. Linux and macOS MUST provide equivalent configuration,
selection, installation, and environment-switching behavior.

Rationale: a single portable binary keeps installation reproducible and makes behavior consistent
across developer workstations.

### II. Safe Shell Integration
Interactive use MUST support Bash and Zsh. Because a child process cannot modify its parent shell,
environment changes MUST be applied through an installed `env-switcher` shell function or another
explicitly sourced shell integration. The CLI MUST emit or expose environment changes in a format
that the integration can apply without using unsafe, unvalidated arbitrary evaluation.

Installation into shell startup files MUST be explicit, idempotent, limited to a clearly marked
managed block, and preserve unrelated user content. Before modifying an existing startup file, the
installer MUST create a recoverable backup or provide an equivalent rollback mechanism. Unsupported
shells MUST fail with an actionable error and MUST NOT be modified.

Rationale: shell initialization files are user-owned and environment export is security-sensitive;
integration must be reversible and must never corrupt an interactive shell.

### III. Configuration Is the Source of Truth
`~/.env-switcher/settings.yaml` MUST be the authoritative definition of shared shell functions,
projects, project directories, environment variables, and project-specific shell functions. The
configuration schema MUST be documented, versionable, deterministically parsed, and validated
before any values are applied. Unknown, duplicate, malformed, or unsafe entries MUST produce clear
errors; partial application is forbidden.

Shell functions defined in settings MUST be treated as trusted user-provided executable code. Their
names, permissions, size, encoding, and target-shell syntax MUST be validated without claiming that
arbitrary code is semantically safe. Function bodies MUST NOT execute during viewing, editing,
validation, reload, hashing, or change detection. They MAY execute only after the user explicitly
selects an environment.

Writes to configuration or managed state under `~/.env-switcher` MUST be atomic. Reload MUST replace
the in-memory project list only after the complete file passes validation. Paths containing `~`
MUST be expanded consistently on Linux and macOS without altering the stored configuration.

Rationale: one validated source of truth prevents a shell from being left with a partially switched
or internally inconsistent environment.

### IV. Secrets Stay Local and Protected
Configuration can contain credentials and MUST be treated as sensitive. `~/.env-switcher` MUST be
created with user-only access, and files containing environment values MUST use user-only read/write
permissions where the platform supports POSIX permissions.

F2 and F3 are the only approved intentional local secret-disclosure paths. F2 MUST display a short
sensitive-data warning and require the user to continue before showing the complete unmasked
`~/.env-switcher/settings.yaml`. F3 MAY expose the complete file through the user-selected editor as
an explicit editing action. Cancellation before either action MUST disclose nothing.

Outside those explicit F2/F3 actions, secret values MUST NOT appear in logs, errors, diagnostics,
crash reports, shell-integration diagnostic output, process arguments, generated examples, or test
fixtures. Displayed or edited F2/F3 content MUST NOT be copied into any prohibited channel. Shell
activation output MAY contain safely quoted secret literals only when required to apply the selected
environment; it MUST be captured by the wrapper, MUST NOT be printed as diagnostic output, and MUST
NOT be persisted by env-switcher.

The project MUST NOT claim that plaintext YAML is a secure secret store. Documentation MUST
recommend references to a dedicated secret manager where practical and MUST clearly describe the
local plaintext risk. Generated examples MUST use non-functional placeholders only.

Rationale: environment variables commonly contain production credentials, so safe defaults and
honest limitations are mandatory.

### V. Tested and Predictable Terminal UX
The CLI MUST provide a keyboard-driven TUI that works in supported Bash and Zsh terminals and
remains usable without mouse input. The following key contracts MUST remain stable unless changed
through a documented breaking release: `F10` exits, `F2` warns and then views the complete unmasked
`~/.env-switcher/settings.yaml` after confirmation, `F3` opens that file in the resolved default
editor, `F4` validates and reloads the project list, and `F5` installs or updates the shell
integration.

The TUI MUST warn that configured shell functions are trusted executable code on first run and
whenever their definitions differ from the last acknowledged set. Warning acknowledgement state
MUST NOT contain function bodies or secret values and MUST be stored with user-only access.

Every action MUST provide a visible success or actionable failure state. Exit codes MUST be stable
and meaningful. Unit tests MUST cover configuration parsing, validation, environment calculation,
and shell integration generation. Integration tests MUST cover Bash and Zsh behavior on Linux and
macOS in CI, including installation idempotency and rollback safety.

Rationale: changing a developer's active environment is high-impact; deterministic behavior and
cross-platform tests are non-negotiable.

## Product and Platform Constraints

- Supported operating systems are Linux and macOS; supported interactive shells are Bash and Zsh.
- Runtime state and the executable installed for the user MUST reside under `~/.env-switcher` unless
  an explicit, documented override is provided.
- Selecting a project MUST apply the complete validated environment for that project, including
  shared definitions, and MUST define deterministic precedence between shared and project-specific
  values. The precedence contract MUST be documented before implementation.
- Project selection MUST NOT silently retain variables managed by the previously selected project.
  The implementation MUST track managed variable names and unset obsolete managed values without
  modifying unrelated user variables.
- Shell-function names and bodies from YAML MUST be validated before installation or execution.
  Arbitrary configuration content MUST NOT be evaluated merely to discover environment values.
- Viewing, editing, validation, reload, hashing, and trusted-code change detection MUST NOT execute
  configured shell functions; only explicit environment selection MAY apply them.
- The default editor MUST be resolved using documented precedence, beginning with user-controlled
  editor environment variables and ending in an explicit actionable error when no editor exists.
- TUI behavior MUST account for terminals that do not deliver function keys reliably by providing
  documented alternative key bindings or commands.
- Dependencies MUST be pinned, actively maintained, and justified; the Go standard library is
  preferred when it provides a clear and reliable solution.

## Development Workflow and Quality Gates

1. Every feature MUST begin with acceptance criteria covering successful behavior, invalid input,
   security-sensitive behavior, and Linux/macOS plus Bash/Zsh compatibility where applicable.
2. Changes MUST pass `gofmt`, static analysis (`go vet` at minimum), unit tests, and integration
   tests relevant to the modified behavior.
3. Shell-startup-file changes and environment emission MUST receive explicit security review,
   including injection, quoting, permissions, idempotency, backup, and rollback cases.
4. Configuration schema changes MUST include compatibility handling, migration notes, and fixtures
   for both the previous and new supported schema versions.
5. Releases MUST publish checksums and MUST identify supported OS/architecture targets. Installation
   documentation MUST include verification and uninstall/rollback steps.
6. Pull requests MUST state which constitutional principles are affected and provide evidence for
   the applicable quality gates. Any exception requires written rationale and a removal plan.

## Governance

This constitution is the highest-priority engineering policy for env-switcher. Specifications,
plans, tasks, implementation, and reviews MUST comply with it. When another project document
conflicts with this constitution, this constitution takes precedence.

Amendments MUST be proposed as a documented change that explains the motivation, compatibility and
security impact, migration requirements, and affected principles. An amendment becomes effective
only after maintainer approval and after dependent specifications or plans are updated.

Constitution versions follow semantic versioning: MAJOR for removal or incompatible redefinition of
a principle, MINOR for a new principle or materially expanded governance, and PATCH for
non-semantic clarification. Every compliance review MUST verify the version, dates, and Sync Impact
Report. Complexity that conflicts with portability, safety, or testability MUST be rejected unless
an amendment explicitly authorizes it.

**Version**: 2.0.0 | **Ratified**: 2026-08-23 | **Last Amended**: 2026-08-23
