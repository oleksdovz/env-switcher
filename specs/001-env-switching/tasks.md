# Tasks: Terminal Environment Switching

**Input**: Design documents from `specs/001-env-switching/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the specification, constitution, and user request. Within each user story,
write the listed tests first and confirm they fail for the expected missing behavior before adding
implementation.

**Organization**: Tasks are ordered by dependency and grouped by independently testable user story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel after its phase prerequisites are complete.
- **[Story]**: Maps the task to its user story from `spec.md`.
- Every task names the file or directory it changes.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize the Go project, dependency policy, command boundary, and test layout.

- [X] T001 Create the planned directories and package documentation files under `cmd/env-switcher/`, `internal/`, `testdata/`, and `tests/`
- [X] T002 Initialize the Go 1.26 module and pin compatible Bubble Tea v2, Bubbles v2, Lip Gloss v2, and YAML v3 dependencies in `go.mod` and `go.sum`
- [X] T003 [P] Add formatting, `go vet`, unit-test, race-test, and build targets to `Makefile`
- [X] T004 [P] Define ignored build, coverage, temporary-home, and local-secret artifacts in `.gitignore`
- [X] T005 [P] Add the minimal executable entry point with build metadata injection points in `cmd/env-switcher/main.go`
- [X] T006 [P] Add shared test helpers that create isolated homes and prohibit access to the real home in `internal/testutil/home.go`
- [X] T007 Add the initial command dispatcher and stable outcome categories in `internal/app/app.go` and `internal/app/errors.go`

**Checkpoint**: The empty application builds on Linux and macOS and has repeatable local quality
commands without implementing a user story.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Implement the strict, secure configuration and domain foundation used by every story.

**⚠️ CRITICAL**: No user story implementation begins until this phase passes its tests.

### Foundational Tests

- [X] T008 [P] Create valid two-project, shell-specific function, and shared-override fixtures in `testdata/config/valid/`
- [X] T009 [P] Create malformed, duplicate-key, alias, merge-key, multi-document, unknown-field, invalid-name, oversized, and unsafe-permission fixtures in `testdata/config/invalid/`
- [X] T010 [P] Add table tests for typed settings entities, limits, identifiers, reserved names, and cross-kind conflicts in `internal/config/model_test.go`
- [X] T011 [P] Add parser contract tests for strict YAML rejection, line/column diagnostics, and full-document atomicity in `internal/config/parser_test.go`
- [X] T012 [P] Add redaction tests proving configured values and function bodies never enter logs, errors, diagnostics, crash reports, or shell-integration diagnostics in `internal/app/errors_test.go`
- [X] T013 [P] Add fuzz tests for YAML decoding limits, malformed UTF-8, and panic resistance in `internal/config/fuzz_test.go`

### Foundational Implementation

- [X] T014 Define Settings, SharedEnvironment, ProjectEnvironment, variable, function-source, and source-position types in `internal/config/model.go`
- [X] T015 Implement bounded YAML node loading and rejection of unsupported YAML constructs in `internal/config/parser.go`
- [X] T016 Implement typed decoding with unknown-field, duplicate-key, schema-version, identifier, size, and cross-kind validation in `internal/config/validate.go`
- [X] T017 [P] Implement user-home expansion, canonical settings paths, ownership checks, and `0700`/`0600` permission enforcement in `internal/config/paths.go`
- [X] T018 Implement scalar/per-shell function selection, deterministic function-definition digesting, and trusted-code change detection without execution in `internal/config/functions.go`
- [X] T019 Implement structured redacted diagnostics and stable exit-code mapping in `internal/app/errors.go`
- [X] T020 Implement first-run atomic creation of `~/.env-switcher/settings.yaml` from a two-project placeholder fixture in `internal/config/bootstrap.go` and `testdata/config/starter.yaml`
- [X] T021 Wire the `validate` command through complete configuration loading without executing functions or producing payload output in `internal/app/validate.go`
- [X] T022 Run the Phase 2 tests and document the strict-configuration gate in `specs/001-env-switching/checklists/foundation-validation.md`

**Checkpoint**: A complete settings document can be created, loaded, rejected, or validated without
partial state, secret disclosure, or shell execution.

---

## Phase 3: User Story 1 - Switch Project Environment (Priority: P1) 🎯 MVP

**Goal**: Select a project and transactionally apply its directory, variables, and functions to the
current Bash or Zsh while cleaning obsolete managed names.

**Independent Test**: In isolated Bash and Zsh sessions, activate two projects in sequence and prove
correct precedence, literal values, directory changes, obsolete-name cleanup, collision behavior,
unrelated-name preservation, payload rejection, and rollback after a forced failure.

### Tests for User Story 1

- [X] T023 [P] [US1] Add deterministic merge and managed-name lifecycle unit tests in `internal/environment/resolve_test.go`
- [X] T024 [P] [US1] Add adversarial literal-quoting table and fuzz tests covering quotes, whitespace, metacharacters, newlines, empty strings, invalid UTF-8, and NUL in `internal/shell/quote_test.go` and `internal/shell/quote_fuzz_test.go`
- [X] T025 [P] [US1] Add v1 envelope, wrong-shell, truncation, extra-byte, size-limit, and redaction contract tests in `tests/contract/payload_test.go`
- [X] T026 [P] [US1] Add Bash and Zsh golden payload fixtures for shared/project precedence and cleanup in `testdata/payload/bash/` and `testdata/payload/zsh/`
- [X] T027 [P] [US1] Add real Bash integration tests including exactly 100 alternating switches, variable/function cleanup, collisions, cancellation, and rollback in `tests/integration/bash_activation_test.go`
- [X] T028 [P] [US1] Add real Zsh integration tests including exactly 100 alternating switches, shell-specific functions, cleanup, cancellation, and rollback in `tests/integration/zsh_activation_test.go`

### Implementation for User Story 1

- [X] T029 [P] [US1] Define immutable EffectiveEnvironment, managed-name sets, and activation-operation types in `internal/environment/model.go`
- [X] T030 [US1] Implement shared/project merging, exact-name precedence, deterministic sorting, and obsolete managed-name calculation in `internal/environment/resolve.go`
- [X] T031 [P] [US1] Implement audited Bash/Zsh-compatible literal quoting and identifier rejection in `internal/shell/quote.go`
- [X] T032 [P] [US1] Define the v1 payload envelope, transaction metadata, limits, and shell-neutral operation contract in `internal/shell/contract.go`
- [X] T033 [US1] Implement Bash payload rendering with state snapshot, `cd`, cleanup, definitions, exports, commit, and rollback in `internal/shell/bash.go`
- [X] T034 [US1] Implement Zsh payload rendering with equivalent transactional semantics in `internal/shell/zsh.go`
- [X] T035 [US1] Implement non-executing target-shell syntax validation for trusted function bodies in `internal/shell/syntax.go`
- [X] T036 [US1] Implement payload-mode orchestration with stdout/stderr separation and no argv secrets in `internal/app/activate.go`
- [X] T037 [US1] Implement minimal Bash/Zsh wrapper templates that verify exit status and envelope framing before evaluation in `internal/install/templates/bash.sh.tmpl` and `internal/install/templates/zsh.sh.tmpl`
- [X] T038 [US1] Wire project selection and internal activation arguments into the command dispatcher in `internal/app/app.go`
- [X] T039 [US1] Run US1 unit, fuzz, golden, and real-shell tests and record MVP evidence in `specs/001-env-switching/checklists/us1-validation.md`

**Checkpoint**: User Story 1 is a testable MVP when its wrapper is sourced manually in an isolated
shell; it does not depend on TUI management or automatic installation.

---

## Phase 4: User Story 2 - Manage and Reload Configuration (Priority: P2)

**Goal**: View, edit, validate, and atomically reload settings while retaining the last valid model
after errors.

**Independent Test**: Open a valid settings file through the documented actions, edit it to valid and
invalid forms, and prove only a fully valid reload replaces the project list while secrets remain
redacted.

### Tests for User Story 2

- [X] T040 [P] [US2] Add default-editor precedence, argument parsing, missing-editor, and failed-editor tests in `internal/editor/editor_test.go`
- [X] T041 [P] [US2] Add atomic reload tests for valid replacement, invalid retention, concurrent change, and removed focus in `internal/app/reload_test.go`
- [X] T042 [P] [US2] Add F2 tests for sensitive-data warning, explicit continuation, full unmasked local view, cancellation, and absence of copied values in diagnostics in `internal/app/settings_test.go`
- [X] T043 [P] [US2] Add temporary-home tests proving first-run/function-change warnings, zero function execution during bootstrap/validate/F2/F3/reload, and digest-only acknowledgement metadata with current-user ownership, `0600` mode, atomic replacement, malformed-state fail-closed behavior, and no secret values or function bodies in `tests/integration/config_workflow_test.go`

### Implementation for User Story 2

- [X] T044 [P] [US2] Implement `VISUAL` then `EDITOR` resolution and direct process execution without a shell in `internal/editor/editor.go`
- [X] T045 [P] [US2] Implement confirmed F2 sensitive-data warning and complete unmasked local settings view with redacted errors in `internal/app/settings.go`
- [X] T046 [US2] Implement non-executing candidate reload, user-only persisted function-digest acknowledgement, trusted-code warning state, and atomic model replacement in `internal/app/reload.go`
- [X] T047 [US2] Add user-visible `validate`, `view`, `edit`, and `reload` command wiring in `internal/app/app.go`
- [X] T048 [US2] Run the independent configuration-management workflow and record evidence in `specs/001-env-switching/checklists/us2-validation.md`

**Checkpoint**: User Story 2 works through non-TUI commands and retains prior valid state on every
reload failure.

---

## Phase 5: User Story 3 - Install Shell Integration (Priority: P3)

**Goal**: Explicitly install, update, roll back, and uninstall one managed Bash/Zsh integration block
without corrupting user profiles.

**Independent Test**: In isolated homes, install twice into representative Bash/Zsh profiles, prove
byte-preservation and one-block idempotency, restore a verified backup, then uninstall while keeping
settings and backups.

### Tests for User Story 3

- [X] T049 [P] [US3] Add managed-block golden fixtures for absent, valid, duplicate, partial, reversed, and nested markers in `testdata/profiles/`
- [X] T050 [P] [US3] Add block reconciliation and unrelated-byte preservation tests in `internal/install/block_test.go`
- [X] T051 [P] [US3] Add backup metadata, digest, mode, target-scope, and rollback validation tests in `internal/install/backup_test.go`
- [X] T052 [P] [US3] Add atomic-write, interruption, lock contention, read-only parent, and symlink rejection tests in `internal/install/filesystem_test.go`
- [X] T053 [P] [US3] Add isolated Bash install/update/rollback/uninstall integration tests in `tests/integration/bash_install_test.go`
- [X] T054 [P] [US3] Add isolated Zsh install/update/rollback/uninstall integration tests in `tests/integration/zsh_install_test.go`

### Implementation for User Story 3

- [X] T055 [P] [US3] Define installation target, managed block, backup metadata, operation result, and lock types in `internal/install/model.go`
- [X] T056 [US3] Implement shell/profile discovery, explicit override validation, ownership checks, and symlink refusal in `internal/install/targets.go`
- [X] T057 [US3] Implement exact managed-block parsing, reconciliation, and malformed-marker failure in `internal/install/block.go`
- [X] T058 [US3] Implement user-only backup creation, metadata serialization, SHA-256 verification, and retention-safe lookup in `internal/install/backup.go`
- [X] T059 [US3] Implement exclusive locking, same-directory temporary writes, mode preservation, fsync, and atomic rename in `internal/install/filesystem.go`
- [X] T060 [US3] Implement idempotent executable/profile installation and post-write verification in `internal/install/install.go`
- [X] T061 [US3] Implement verified rollback and conservative uninstall that preserves settings/backups by default in `internal/install/rollback.go` and `internal/install/uninstall.go`
- [X] T062 [US3] Wire confirmation, no-op reporting, backup IDs, rollback, uninstall, and recovery guidance into `internal/app/install.go`
- [X] T063 [US3] Run Bash/Zsh install lifecycle tests and record evidence in `specs/001-env-switching/checklists/us3-validation.md`

**Checkpoint**: User Story 3 installs the US1 wrapper safely and reversibly without requiring the TUI.

---

## Phase 6: User Story 4 - Navigate the Terminal Interface (Priority: P4)

**Goal**: Provide the keyboard-driven Bubble Tea experience for project selection, settings actions,
installation, status, cancellation, and function-key alternatives.

**Independent Test**: In PTY tests, use only documented keys to navigate, select, view, edit, reload,
install, cancel, and exit in normal and small terminals without accidental activation or secret output.

### Tests for User Story 4

- [X] T064 [P] [US4] Add model/update tests for focus, Enter confirmation, trusted-function warnings, F2 sensitive-data confirmation, cancellation, and stable key bindings in `internal/tui/model_test.go`
- [X] T065 [P] [US4] Add view tests for footer actions, visible focus, statuses, redaction, empty lists, and small terminal layouts in `internal/tui/view_test.go`
- [X] T066 [P] [US4] Add command tests for editor suspend/resume, reload retention, install confirmation, and payload-on-confirmation only in `internal/tui/update_test.go`
- [X] T067 [P] [US4] Add Bash/Zsh PTY acceptance tests proving project switching within 10 seconds and three deliberate interactions plus all function-key alternatives in `tests/integration/tui_test.go`

### Implementation for User Story 4

- [X] T068 [P] [US4] Define Bubble Tea messages, model state, selection state, and key map in `internal/tui/model.go` and `internal/tui/keys.go`
- [X] T069 [P] [US4] Implement responsive project list, focus, help footer, empty/error/status views, and secret-safe rendering in `internal/tui/view.go`
- [X] T070 [US4] Implement update handling for navigation, Enter, F2 sensitive-data confirmation, trusted-function warnings, F3-F5, F10, alternatives, and cancellation in `internal/tui/update.go`
- [X] T071 [US4] Integrate editor suspend/resume and service commands for view/reload/install actions in `internal/tui/commands.go`
- [X] T072 [US4] Connect TUI completion to payload preparation while keeping stdout machine-only in `internal/app/tui.go`
- [X] T073 [US4] Run the PTY acceptance suite and record independent US4 evidence in `specs/001-env-switching/checklists/us4-validation.md`

**Checkpoint**: All four stories are independently testable and integrated through the final TUI.

---

## Phase 7: Polish & Cross-Cutting Production Readiness

**Purpose**: Validate the full security, compatibility, documentation, and release surface.

- [X] T074 [P] Add Linux Bash/Zsh CI jobs with formatting, vet, unit, fuzz smoke, race, integration, and built-binary tests in `.github/workflows/ci-linux.yml`
- [X] T075 [P] Add macOS Bash/Zsh CI jobs and architecture build checks in `.github/workflows/ci-macos.yml`
- [X] T076 [P] Add cross-build, version metadata, SHA-256 checksum, and four-target artifact generation in `.github/workflows/release.yml`
- [X] T077 [P] Add native artifact version/validate/checksum smoke tests in `tests/release/artifact_test.go`
- [X] T078 Add secret-canary tests proving zero disclosure through logs, errors, diagnostics, crash reports, user-visible shell-integration output, child-process argument vectors, and persisted metadata while explicitly excluding confirmed F2 and user-directed F3 in `tests/integration/secret_redaction_test.go`
- [X] T079 Add a hard-fail CI performance test for reload of 100 projects with 100 variables and 100 functions per project within 2 seconds on Linux and macOS, retain secret-free timing evidence, and add payload-preparation benchmarks in `internal/config/performance_test.go`, `internal/shell/benchmark_test.go`, `.github/workflows/ci-linux.yml`, and `.github/workflows/ci-macos.yml`
- [X] T080 [P] Document F2/F3 intentional disclosure, first-run/function-change warnings, trusted executable functions, configuration, and secret-manager guidance in `README.md` and `docs/configuration.md`
- [X] T081 [P] Document validation, rollback, uninstall, troubleshooting, protocol compatibility, and security boundaries in `docs/operations.md` and `docs/security.md`
- [X] T082 Execute every scenario in `specs/001-env-switching/quickstart.md` on Linux and macOS and record actual evidence in `specs/001-env-switching/checklists/quickstart-validation.md`
- [X] T083 Run a final constitution/security review covering quoting, permissions, injection, atomic writes, backup restoration, checksums, and release targets in `specs/001-env-switching/checklists/release-readiness.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 Setup** starts immediately.
- **Phase 2 Foundation** depends on Phase 1 and blocks all user stories.
- **US1 (Phase 3)** starts after Phase 2 and is the MVP.
- **US2 (Phase 4)** starts after Phase 2; its non-TUI commands are independently testable, while its
  later TUI exposure is completed in US4.
- **US3 (Phase 5)** starts after Phase 2; its installer consumes the wrapper contract from US1, so
  T060-T063 depend on T037 while its tests and lower-level install components may start earlier.
- **US4 (Phase 6)** depends on the application services delivered by US1, US2, and US3.
- **Phase 7** follows all stories selected for release; CI scaffolding tasks can begin earlier once
  their referenced commands exist.

### User Story Dependency Graph

```text
Setup -> Foundation -> US1 (MVP) -----------+
                     -> US2 ----------------+-> US4 -> Production readiness
                     -> US3 core -> US1 wrapper -> US3 integration -+
```

### Within Each Story

1. Write contract, unit, fuzz, golden, and integration tests first; confirm expected failures.
2. Add entities and shell-neutral logic.
3. Add shell/platform-specific implementation.
4. Wire the application boundary.
5. Run the independent test and record evidence before moving forward.

## Parallel Opportunities

- Phase 1 tasks T003-T006 can run concurrently after T002 where dependency imports are needed.
- Phase 2 fixtures/tests T008-T013 can run concurrently; T017 can proceed alongside T014-T016.
- After Phase 2, US1 test fixtures, US2 editor/reload tests, and US3 installation fixtures can be
  authored in parallel.
- Bash and Zsh tests/renderers are parallel pairs once the shared contract exists.
- Documentation, artifact smoke tests, and platform CI definitions use separate files in Phase 7.

## Parallel Example: User Story 1

```text
T023 resolve lifecycle tests
T024 quoting table/fuzz tests
T025 envelope rejection tests
T026 Bash/Zsh golden fixtures
T027 Bash integration tests
T028 Zsh integration tests
```

After T029-T032, T033 and T034 can proceed in parallel before T036-T039.

## Parallel Example: User Story 2

```text
T040 editor tests
T041 reload tests
T042 view tests
T043 temporary-home workflow tests
```

T044 and T045 can proceed in parallel before T046-T048.

## Parallel Example: User Story 3

```text
T049 profile fixtures
T050 block tests
T051 backup tests
T052 filesystem tests
T053 Bash install integration tests
T054 Zsh install integration tests
```

After T055, block, backup, and filesystem implementation should follow their respective failing tests;
Bash and Zsh integration validation can then run in parallel.

## Parallel Example: User Story 4

```text
T064 model/key tests
T065 view tests
T066 command tests
T067 PTY acceptance tests
```

T068 and T069 can proceed in parallel before T070-T073.

## Implementation Strategy

### MVP First

1. Complete Setup and Foundation.
2. Complete US1 through T039.
3. Source the generated wrapper manually only in isolated Bash/Zsh sessions.
4. Stop and validate current-shell switching, cleanup, collision behavior, rollback, quoting, and
   redaction before adding installation or TUI convenience.

### Incremental Delivery

1. **US1**: current-shell switching engine and wrapper contract.
2. **US2**: safe configuration lifecycle through non-TUI commands.
3. **US3**: reversible installation lifecycle.
4. **US4**: complete keyboard TUI over proven services.
5. **Production readiness**: cross-platform CI, releases, documentation, and final security evidence.

## Notes

- `[P]` means the task touches independent files and has no incomplete logical prerequisite.
- Tests must use temporary homes and must never mutate the developer's actual shell profiles.
- Inline function bodies are trusted local code; tasks must not claim semantic sanitization.
- Successful switching intentionally does not restore user definitions overwritten by env-switcher.
- Do not commit, push, publish, install into a real home, or release without explicit approval.
