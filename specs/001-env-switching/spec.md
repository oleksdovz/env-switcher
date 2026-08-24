# Feature Specification: Terminal Environment Switching

**Feature Branch**: `001-env-switching`

**Created**: 2026-08-23

**Status**: Draft

**Input**: User description: "Create env-switcher, a Linux and macOS terminal application with a
keyboard-driven interface that manages project environments from ~/.env-switcher/settings.yaml,
supports Bash and Zsh, and installs shell integration for applying the selected environment."

## Clarifications

### Session 2026-08-23

- Q: What should env-switcher do when a managed variable or shell function already exists in the
  current shell but was not created by env-switcher? → A: Overwrite it, then remove it when the
  next selected project no longer defines it; do not restore the original value or function.
- Q: How should the env-switcher command transfer the selected environment from the TUI to the
  current Bash or Zsh process? → A: The CLI emits a well-defined shell payload for a minimal Bash
  or Zsh wrapper to evaluate after a successful CLI exit.
- Q: How should F2 handle secrets stored in settings.yaml? → A: After a short sensitive-data
  warning, F2 intentionally shows the complete local file without masking, including secret values.
- Q: What is the trust model for configured shell functions? → A: Shell functions are trusted
  user-provided executable code; warn on first run and after function changes, and execute them only
  after explicit environment selection.

### Session 2026-08-24

- Q: The installed shell function was found to reactivate the last-switched project (re-running
  its `shell-cmd` hooks, redefining its functions) after *any* successful command, including
  `--help`. What should the shell payload contract be? → A: The current-environment file is
  written only by the three actions that actually switch a project (bare invocation, a project
  name, `--select`); the installed shell function clears it before every invocation and sources it
  afterward only if that invocation just rewrote it, so every other command — including a failed
  switch attempt — leaves the running shell exactly as it was, never re-applying a stale prior
  switch.
- Q: How should env-switcher upgrade itself? → A: A documented `upgrade`/`--upgrade` command (and
  `F6` in the terminal interface, driving the identical service) checks the latest **stable**
  (non-draft, non-prerelease) release of `oleksdovz/env-switcher` by semantic-version comparison,
  downloads the asset matching the running platform, verifies it against the release's published
  checksums, and atomically replaces the installed executable — never installing an unverified or
  partially-downloaded binary, and never touching the existing installation if any step fails.
- Q: Users were manually declaring a `PROJECT_DIR`/`_PROJECT`-style `env-vars` entry to get their
  project's directory into a variable, then wanted to reference it from other variables — should
  env-switcher provide this automatically? → A: Yes. `_PROJECT` becomes a reserved, automatically
  populated variable: `project` is resolved (expanding a leading `~`/`~/` or `$HOME`/`${HOME}` —
  the application's one controlled, no-shell expansion mechanism — into a clean absolute path,
  failing the switch if that's not possible) and exposed as `_PROJECT` on every switch. Other
  `env-vars` values may reference it via `$_PROJECT`/`${_PROJECT}` substitution (and nothing else
  is expanded that way); shared/project shell functions and shell-cmd see it as an ordinary
  exported shell variable. A manually declared `_PROJECT` is accepted (not an error, for
  settings written before this existed) but always overwritten by the computed value.
- Q: Should a shell function name like `k-load` (rejected as an "invalid identifier") be allowed?
  → A: Yes — that was a genuine bug. A shell *variable* name must be a plain POSIX identifier, but
  a shell *function* name is just a command name to bash/zsh, and both legitimately accept
  hyphens/dots/colons between segments. Function names now follow a superset rule; variable names
  are unchanged.
- Q: `--install`, `--validate`, `--reload`, `--view`, `--rollback`, and `--uninstall` all failed
  with "project ... is not configured" even though every command is documented as accepting an
  equivalent `--flag` form. → A: Genuine bug — the dispatcher's switch-cases for those six commands
  only matched the bare word, so their `--flag` form silently fell through to the "switch to a
  project named X" default case. Fixed to match both forms for every documented command; a
  regression test now asserts each one does.
- Q: `env-switcher upgrade` failed outright on a locally built ("dev") binary, since "dev" isn't a
  semantic version to compare against — should it? → A: No — that left dev builds with no way to
  reach a release at all. An unparseable running version is now treated as definitely not current,
  so the latest stable release is always installed over it (see FR-037).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Switch Project Environment (Priority: P1)

As a developer, I can select a configured project in the terminal and apply its environment to my
current shell so that I can work on different projects without manually exporting variables or
changing directories.

**Why this priority**: Environment switching is the primary user value; without it the product does
not solve its core problem.

**Independent Test**: Configure two projects with distinct directories and variables, select each
one in turn, and verify that the current shell contains exactly the selected project's managed
values while unrelated shell values remain unchanged.

**Acceptance Scenarios**:

1. **Given** two valid configured projects and installed shell integration, **When** the user selects
   the first project, **Then** its environment variables, shared definitions, and project-specific
   definitions become available in the current shell. Switching never changes the shell's current
   working directory; `project` is informational metadata only (shown by `list`/`get`), not a `cd`
   target.
2. **Given** the first project is active, **When** the user selects the second project, **Then** the
   second project's variables and functions are applied on top of whatever is already in the
   shell; anything the first project set that the second doesn't define is left as-is, not removed.
3. **Given** a variable or function already exists but is not managed by env-switcher, **When** a
   selected project defines the same name, **Then** env-switcher overwrites it without preserving
   the original definition.
4. **Given** a project defines the same managed value as a shared definition, **When** that project
   is selected, **Then** the project-specific value takes precedence.
5. **Given** a configured project's directory does not exist, **When** the user switches to it,
   **Then** the switch still succeeds — a nonexistent or unreadable `project` path is not a
   switching precondition.
6. **Given** the CLI successfully prepares a selected environment, **When** it finishes, **Then** the
   shell-specific activation script is written to the fixed current-environment file and the
   installed shell function sources that file in the current shell.
7. **Given** the CLI fails to prepare a selected environment, **When** control returns to the shell
   function, **Then** the current-environment file is left unchanged, so sourcing it again reapplies
   the prior state and the current environment does not change.
8. **Given** a project was successfully switched to earlier in the current shell session, **When**
   the user runs any command that does not itself switch a project (e.g. `--help`, `list`, `get`,
   `version`, `validate`, `reload`, `view`, `edit`, `install`, `upgrade`, `rollback`, `uninstall`,
   or a switch attempt that fails), **Then** that command's own output is printed once, exactly as
   it would be standalone, and the shell function does not re-source the current-environment file,
   re-run any `shell-cmd` hook, redefine any shell function, change directory, or modify any
   environment variable.

---

### User Story 2 - Manage and Reload Configuration (Priority: P2)

As a developer, I can view, edit, validate, and reload the settings file from the terminal interface
so that project changes are available without restarting my shell.

**Why this priority**: Users need a reliable way to maintain the source of truth before environment
switching remains useful over time.

**Independent Test**: Start with a valid settings file, view and edit it through the documented
actions, reload it, and verify that the displayed project list changes only after successful
validation.

**Acceptance Scenarios**:

1. **Given** the terminal interface is open, **When** the user invokes `F2`, **Then** a short warning
   states that the file contains sensitive values and the complete settings file is shown without
   masking only after the user continues.
2. **Given** the F2 sensitive-data warning is visible, **When** the user cancels, **Then** no settings
   content is displayed or copied to any output channel.
3. **Given** a default editor is available, **When** the user invokes `F3`, **Then** the settings file
   opens in that editor and the terminal interface can continue after the editor closes.
4. **Given** the settings file contains two valid projects, **When** the user invokes `F4`, **Then**
   both projects appear in the refreshed project list.
5. **Given** the edited settings file is invalid, **When** the user invokes `F4`, **Then** the previous
   valid project list remains active and validation errors identify the affected setting without
   revealing secret values.
6. **Given** no default editor can be resolved, **When** the user invokes `F3`, **Then** the file is
   unchanged and the user receives instructions for configuring an editor.
7. **Given** settings contain shell functions, **When** the user views, edits, validates, or reloads
   settings, **Then** no configured function body is executed.
8. **Given** shell functions exist on first run or their definitions have changed, **When** settings
   are loaded, **Then** the user is warned that selecting an environment executes trusted local code.
9. **Given** the user has acknowledged the current trusted-function warning, **When** unchanged
   function definitions are loaded in a later run, **Then** no repeated warning is required.

---

### User Story 3 - Install Shell Integration (Priority: P3)

As a developer, I can install env-switcher integration into my current supported shell so that the
`env-switcher` command can update the environment of that shell in future sessions, and I can get
that done just by running whatever copy of the binary I have, without memorizing a subcommand.

**Why this priority**: Installation makes the primary flow convenient and persistent, but it can be
tested independently from configuration editing.

**Independent Test**: Install the integration twice in clean Bash and Zsh profiles, start new shell
sessions, and verify that exactly one managed integration block exists and switching works in each
session. Separately, run a never-before-seen executable path with no arguments against a clean home
and verify the first-run prompt appears before any file is created, and that running it again from
elsewhere afterward updates silently.

**Acceptance Scenarios**:

1. **Given** the user is running Bash or Zsh, **When** the user invokes `F5` (or the documented
   `env-switcher install` command) and confirms installation, **Then** the executable and a clearly
   marked shell integration are installed under the documented user-owned locations.
2. **Given** integration is already installed, **When** installation runs again, **Then** the existing
   managed integration is updated without adding duplicates or modifying unrelated profile content.
3. **Given** an existing shell profile will be changed, **When** installation begins, **Then** a
   recoverable copy or equivalent rollback mechanism is available before the change is committed.
4. **Given** an unsupported shell is active, **When** installation is requested, **Then** no profile is
   modified and the supported shells are identified.
5. **Given** the default (no-argument) invocation is run from an executable path that is not the
   installed one, and nothing is installed yet on this machine, **When** the user confirms the
   resulting one-time prompt, **Then** the settings directory, starter settings file, installed
   executable, and shell integration are all created together; declining leaves all of them absent
   and still runs the requested (TUI) action.
6. **Given** the default invocation is run from an executable path that is not the installed one,
   and this machine already has an installed executable, **When** the CLI starts, **Then** it
   silently refreshes the installed executable and, if its template changed, the managed shell
   block, without prompting.
7. **Given** the default invocation is run from the already-installed executable path itself (the
   ordinary case through the installed shell function), **When** the CLI starts, **Then** neither
   of the two self-install behaviors above runs.

---

### User Story 4 - Navigate the Terminal Interface (Priority: P4)

As a keyboard-only user, I can identify available projects and actions, trigger them predictably,
and exit without changing my environment accidentally.

**Why this priority**: The terminal interface improves discoverability and usability after the core
switching and management flows exist.

**Independent Test**: Use only the keyboard to navigate projects, inspect the action labels, cancel
an operation, and exit while verifying that no unconfirmed environment change occurs.

**Acceptance Scenarios**:

1. **Given** the terminal interface is open, **When** the user navigates with documented keyboard
   controls, **Then** the focused project and available actions are visibly distinguishable.
2. **Given** a project is focused but not confirmed, **When** the user exits with `F10`, **Then** the
   interface closes without changing the current environment.
3. **Given** the terminal does not deliver function-key input reliably, **When** the user uses the
   documented alternative binding or command, **Then** the same action is performed.
4. **Given** an operation succeeds or fails, **When** it completes, **Then** the user receives a clear
   status that does not disclose secret values.

### User Story 5 - Upgrade env-switcher (Priority: P3)

As a developer, I can upgrade my installed env-switcher to the latest compatible release from the
terminal interface or the CLI, verified and installed without leaving my installation in a broken
or unverified state.

**Why this priority**: Keeping the tool current matters once it's installed and depended on daily,
but it is independent of — and can be tested independently from — the core switching and
configuration flows.

**Independent Test**: Point the upgrade check at a fixture release server (never live GitHub) with
a newer stable version, run the upgrade, and verify the installed executable, reported old/new
version, and installed path; separately, verify an already-current installation reports that
clearly and changes nothing, and that a corrupted/missing checksum, unsupported platform, or
failed download each leave the previously installed executable untouched.

**Acceptance Scenarios**:

1. **Given** the running version is older than the latest stable release, **When** the user invokes
   `F6` (or the documented `env-switcher upgrade`/`--upgrade` command) and confirms in the terminal
   interface, **Then** the matching platform asset is downloaded, verified against the release's
   published checksums, and atomically installed at the canonical executable path, and the old
   version, new version, and installed path are reported.
2. **Given** the running version is already the latest stable release, **When** an upgrade is
   requested, **Then** no download or installation occurs and the user is told clearly that
   env-switcher is already up to date.
3. **Given** a release is a draft or a prerelease, **When** the latest stable release is determined,
   **Then** that release is excluded from consideration regardless of its version number.
4. **Given** the release the upgrade would install does not publish a checksum file, **When** the
   upgrade runs, **Then** it fails with an actionable error instead of installing an unverified
   binary, and the previously installed executable is left unchanged.
5. **Given** a downloaded asset's checksum does not match the release's published checksum, **When**
   verification runs, **Then** the upgrade fails and the previously installed executable is left
   unchanged.
6. **Given** the release publishes no asset for the running operating system and architecture,
   **When** the upgrade runs, **Then** it fails with an actionable error naming the platform sought
   and the platforms that are actually available, and the previously installed executable is left
   unchanged.
7. **Given** the F6 terminal action and the `upgrade`/`--upgrade` CLI command, **When** either is
   invoked, **Then** both drive the identical upgrade logic; neither reimplements platform
   detection, release retrieval, version comparison, checksum verification, or installation on its
   own.
8. **Given** a project's configured name would collide with the `upgrade` command word, **When**
   settings are validated, **Then** that name is rejected for the same reason other reserved
   command words are.

---

### Edge Cases

- The settings directory or file does not exist on first run.
- The settings file exists but cannot be read because of ownership or permissions.
- The configuration is empty, contains duplicate project names or variable names, or has unknown
  fields.
- A project path contains spaces, uses `~`, does not exist, or is not a directory — none of which
  affect switching, since `project` is informational only.
- A variable or shell-function name is invalid, duplicated, or conflicts across shared and project
  scope.
- A value contains whitespace, quotes, newlines, shell metacharacters, or an empty string.
- A settings change occurs while the interface is open or while a reload is in progress.
- The current shell profile is missing, read-only, a symbolic link, or already contains a partial
  managed integration block.
- Installation is interrupted after backup but before the profile update completes.
- The default editor exits unsuccessfully or changes the settings file to invalid content.
- The user switches repeatedly between projects with overlapping and non-overlapping variables.
- Terminal dimensions are too small to show the complete project list or help text.
- The default invocation runs from a path other than the installed executable location, on a
  machine with no prior installation, with an unsupported or undetected shell, or with a
  first-run confirmation declined.
- A non-switch command runs in a shell session where a project was previously activated, or where
  the current-environment file otherwise still exists from an earlier session.
- A switch attempt (bare invocation, project name, or `--select`) fails validation or resolution.
- The upgrade check has no network access, the release host returns a rate-limit or server error,
  or the request times out.
- The latest release is a draft or a prerelease, or its tag does not parse as a semantic version.
- The latest release publishes no checksum file, an asset the checksum file doesn't list, or a
  checksum that doesn't match the downloaded asset.
- The latest release publishes no asset for the running operating system and architecture.
- A downloaded release archive contains an absolute path, a `..` traversal entry, a symbolic link,
  or any entry other than the single expected executable.
- An earlier installation left an executable at the pre-`bin/` install location.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The product MUST operate in supported Linux and macOS terminal environments and expose
  equivalent user-visible behavior in Bash and Zsh.
- **FR-002**: The product MUST use `~/.env-switcher/settings.yaml` as the default source of shared
  definitions and project environment definitions.
- **FR-003**: On first run, the product MUST be able to create `~/.env-switcher` and a starter
  settings file containing three clearly identified example projects and non-functional placeholder
  credentials.
- **FR-004**: The settings model MUST support shared shell functions and an anonymous shared
  `shell-cmd` hook plus multiple named projects, each with an informational project directory
  (not used for switching), environment variables, project-specific shell functions, and an
  optional project-specific `shell-cmd`.
- **FR-005**: The complete settings file MUST be validated before the product displays reloaded data,
  installs functions, or applies any environment change.
- **FR-006**: Validation MUST reject malformed structure, duplicate identifiers within a scope,
  invalid variable or function names, and unsafe or unsupported entries with field-specific errors.
- **FR-007**: Users MUST be able to view the complete settings file with `F2`, including unmasked
  secret values, only after a short warning that the file contains sensitive data. This intentional
  local view MUST NOT be copied into diagnostics or logs.
- **FR-008**: Users MUST be able to open the settings file with `F3` using their resolved default
  editor and receive an actionable error if no editor is available.
- **FR-009**: Users MUST be able to validate and reload projects with `F4`; a failed reload MUST retain
  the last completely valid in-memory project list.
- **FR-010**: Users MUST be able to install or update shell integration with `F5` after explicit
  confirmation.
- **FR-011**: Installation MUST place or update the user executable under `~/.env-switcher/bin` and
  MUST add only a clearly marked managed integration block to the applicable Bash or Zsh startup
  file. Re-running installation after a binary upgrade MUST refresh that block to the current
  wrapper contents.
- **FR-012**: Installation MUST be idempotent, preserve all unrelated startup-file content, and make
  rollback possible before committing a profile change.
- **FR-013**: A successful project selection MUST write a well-defined, shell-specific activation
  script to a fixed session file (`~/.env-switcher/current-env`), and the installed `env-switcher`
  shell function MUST invoke the CLI and then source that file in the current shell, regardless of
  which documented command or bare project name was used to invoke it.
- **FR-014**: Selecting a project MUST apply all validated managed variables and shell functions as
  one successful operation. Switching MUST NOT change the shell's current working directory, and a
  project's configured directory need not exist for the switch to succeed.
- **FR-015**: Project-specific definitions MUST override shared definitions with the same identifier;
  definitions that do not conflict MUST be combined.
- **FR-016**: Switching MUST NOT remove variables or functions left over from a previously active
  project; each switch only applies the newly selected project's own variables and functions on
  top of the shell's existing state. If env-switcher overwrites a pre-existing variable or function
  with the same name, it MUST NOT preserve or restore the original definition.
- **FR-017**: Applying a selected project's variables/functions/shell-cmd MUST NOT be an
  all-or-nothing transaction: there is no snapshot or automatic rollback. If one name fails to
  apply (e.g. a shell `readonly` conflict), the shell reports its own error for that statement and
  every other statement in the switch still applies.
- **FR-018**: The terminal interface MUST list configured projects, indicate the focused selection,
  display available keyboard actions, and support operation without a mouse.
- **FR-019**: `F10` MUST exit without applying an unconfirmed selection.
- **FR-020**: Every function-key action MUST have a documented alternative binding or command for
  terminals that do not reliably transmit function keys.
- **FR-021**: The settings directory and files containing environment values MUST be restricted to
  the current user where the operating system supports user ownership and permissions.
- **FR-022**: Except for confirmed F2 viewing and user-directed F3 editing, secret values MUST NOT
  appear in routine output, logs, errors, diagnostics, crash reports, user-visible shell-integration
  output, process arguments, or generated example data.
- **FR-023**: The product MUST clearly warn that plaintext settings are not a secure secret store and
  direct users toward external secret-management references for sensitive environments.
- **FR-024**: All file updates managed by the product MUST be atomic or leave the prior complete file
  intact when interrupted.
- **FR-025**: Successful and failed actions MUST produce distinct, stable outcomes suitable for both
  interactive use and automated verification.
- **FR-026**: The activation script MUST be written to the current-environment file only after a
  fully successful project resolution; a failed switch MUST leave any existing file unchanged so
  that sourcing it again reproduces the prior state rather than a partial or invalid one.
- **FR-029**: The product MUST provide a documented CLI command, in both bare-word and `--flag`
  form, to list configured projects (`list`/`ls`), to show one project's complete resolved
  configuration (`get`), to open settings for editing (`edit`), to print usage (`help`), and to
  print build metadata (`version`); a bare word matching none of these MUST be treated as a
  project name to switch to directly.
- **FR-030**: Configured project names MUST be rejected at validation time if they collide with a
  reserved CLI command word, since such a name would be unreachable through the bare-word switch
  form.
- **FR-031**: The default (no-argument) invocation MUST compare the running executable's path
  against the fixed installed-executable location. When they differ and nothing is installed
  there yet, it MUST ask for confirmation before creating the settings directory and starter
  settings file, installing shell integration, and copying itself into place, and MUST take none
  of those actions if declined. When they differ and something is already installed there, it
  MUST perform the same executable and shell-integration refresh without asking. When they match
  (the ordinary case through the installed shell function), it MUST do neither.
- **FR-027**: Shell functions and the anonymous `shell-cmd` hook from settings MUST be treated as
  trusted user-provided executable code; their names (where applicable), permissions, size,
  encoding, and syntax for the active shell MUST be validated without claiming semantic safety. A
  function or `shell-cmd` body has no separate per-shell form; the same body is offered to and
  checked against whichever shell is active.
- **FR-028**: The product MUST warn about trusted code execution (shell functions or `shell-cmd`) on
  first run and after any such definition changes. Bodies MUST NOT execute during F2 view, F3 edit,
  validation, or reload; they may execute only after explicit environment selection. Warning
  acknowledgement MUST persist across runs without storing bodies or secret values.
- **FR-033**: A project's `shell-cmd`, if configured, MUST run after the shared `shell-cmd` (if
  configured) as the last step of a successful switch — additive, not an override — and its exit
  status MUST NOT roll back a switch that has already committed.
- **FR-034**: The settings parser MUST accept YAML anchors, aliases, and merge keys, subject to
  bounded expansion: an anchored value MUST NOT itself reference another anchor, and the document's
  total size after resolving every alias MUST NOT exceed a fixed cap independent of the raw-file
  size cap.
- **FR-035**: Only a bare invocation, a documented project name, or `--select` MUST ever write the
  current-environment file; every other documented command (including a switch attempt that fails)
  MUST leave it untouched. The installed shell function MUST clear the current-environment file
  before invoking the CLI and source it afterward only if that invocation just rewrote it, so a
  non-switch command or a failed switch never re-applies a stale prior switch's variables,
  functions, or `shell-cmd` side effects. Distinguishing an actual switch from routine output MUST
  NOT rely on the CLI's exit code alone.
- **FR-036**: Users MUST be able to upgrade the installed executable with `F6` (terminal interface)
  or the documented `upgrade`/`--upgrade` CLI command; both MUST drive one shared upgrade
  implementation rather than each implementing platform detection, release retrieval, version
  comparison, checksum verification, or installation independently.
- **FR-037**: The upgrade check MUST determine the latest release using semantic-version
  comparison of non-draft, non-prerelease releases only — never simply the most recently published
  release — and MUST report clearly, without downloading or installing anything, when the running
  version is already current. A running version that does not itself parse as a semantic version
  (a local/dev build, e.g. the literal "dev") MUST NOT be reported as already current or refuse to
  compare — it MUST be treated as definitely not current, so the latest stable release is always
  installed over it.
- **FR-038**: The upgrade action MUST select the release asset matching the running operating
  system and architecture, and MUST fail with an actionable error naming both the sought platform
  and the platforms the release actually publishes when no match exists.
- **FR-039**: The upgrade action MUST verify a downloaded asset against the release's published
  checksums before installing it. A release that publishes no checksum file, or a checksum that
  does not match, MUST be treated as a hard failure — never a silently skipped check and never an
  installed-but-unverified binary.
- **FR-040**: When the downloaded asset is an archive, the upgrade action MUST extract only the
  single expected executable entry, rejecting an absolute path, a `..` traversal component, a
  symbolic link, or any other archive entry.
- **FR-041**: The upgrade action MUST install the new executable atomically at the canonical
  install path and MUST leave the previously installed executable unchanged if any step (network
  access, checksum verification, extraction, or installation) fails. On success it MUST report the
  old version, new version, and installed path.
- **FR-042**: `upgrade` MUST be a reserved CLI command word, rejected as a configured project name
  at validation time for the same reason other reserved command words are (FR-030).
- **FR-043**: An executable left over at a pre-`bin/`-convention install location MUST be safely
  migrated (removed only once the canonical `bin/` executable is confirmed in place) rather than
  left as an orphaned, unreferenced copy or a target the shell integration could be pointed at.
- **FR-044**: Every successful switch MUST populate a reserved `_PROJECT` variable with the
  environment's `project` path resolved to a clean absolute path, using only the application's
  controlled `~`/`~/`/`$HOME`/`${HOME}` expansion — never a shell or `eval`. An empty, relative, or
  otherwise unresolved `project` MUST fail the switch.
- **FR-045**: Any `env-vars` value (shared or project-specific) MUST be able to reference the
  resolved project path via `$_PROJECT` or `${_PROJECT}` substitution; no other variable reference
  is expanded in `env-vars` values. Shared and project-specific `shell-functions` and `shell-cmd`
  MUST see `_PROJECT` as an ordinary exported shell variable, without any text substitution into
  their bodies.
- **FR-046**: A manually declared `_PROJECT` under `env-vars`, in either shared or project scope,
  MUST be accepted by validation (so configuration written before this variable existed keeps
  loading) but MUST always be overwritten by the computed value when an environment is resolved —
  a declared value MUST NOT be used and MUST NOT cause a validation error.
- **FR-047**: A shell function name MUST be validated against a rule that accepts the
  hyphen/dot/colon-separated forms bash and zsh both treat as valid command names (e.g.
  `k-load`), not the stricter POSIX-identifier rule required of a variable name (which must remain
  a valid `export` target); the reserved-prefix rule (FR-027) applies to both.

### Key Entities

- **Settings**: The complete validated user configuration, including shared definitions and an
  ordered collection of uniquely named projects.
- **Project Environment**: A named switch target containing an informational project directory
  (not a `cd` target), managed environment variables, and project-specific shell functions.
- **Reserved Project Variable (`_PROJECT`)**: The environment's `project` path, resolved to a
  clean absolute path and automatically populated on every switch — referenceable from other
  `env-vars` values and visible as an ordinary exported shell variable to shell functions and
  shell-cmd; never user-settable, regardless of whether it's declared.
- **Shared Definition**: A function or setting available to every project unless a project-specific
  definition with the same identifier overrides it.
- **Managed Variable**: An environment variable whose lifecycle is controlled by env-switcher,
  including its name, selected value, source scope, and active/inactive state.
- **Shell Function**: A validated named command definition made available through the shell
  integration in shared or project scope; after activation its name is managed under the same
  overwrite-and-remove lifecycle as a managed variable. Its body has no per-shell variant — one
  body is offered to whichever shell is active.
- **Shell Command Hook**: An anonymous, unnamed counterpart to a shell function that runs
  automatically as the last step of a switch rather than being invoked on demand; shared and
  project scope are additive (both run, shared first) rather than override-by-name.
- **Shell Integration**: The user-approved managed profile block and command contract that allow a
  selected environment to affect the current supported shell through a minimal wrapper that
  invokes the CLI and then sources the resulting well-defined, shell-specific activation file.
- **Installation Backup**: A recoverable representation of the shell profile before env-switcher
  modifies it.
- **Release**: A published version of env-switcher identified by a semantic-version tag, marked
  draft/prerelease or not, and offering one downloadable asset per supported operating
  system/architecture plus a checksum file covering them.
- **Upgrade Check**: The shared operation (invoked identically by `F6` and the `upgrade`/`--upgrade`
  command) that compares the running version against the latest non-draft, non-prerelease release
  and, if newer, downloads, verifies, and atomically installs its matching platform asset.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with a valid two-project configuration can switch from one project to the other
  and verify the new managed values in no more than 10 seconds and three deliberate interactions,
  measured by an automated PTY acceptance test from an open project list.
- **SC-002**: Every acceptance test exercising a switch confirms the selected project's own
  variables/functions/shell-cmd are all applied; a name that fails to apply (e.g. a `readonly`
  conflict) does not prevent unrelated statements in the same switch from applying.
- **SC-003**: Repeated installation or update produces exactly one managed integration block and
  preserves 100% of unrelated profile content across supported shells and operating systems.
- **SC-004**: After exactly 100 consecutive switches between two projects in each supported shell,
  every switch applies without error and no unrelated (non-project) shell definition is changed.
- **SC-005**: Valid settings changes become visible through reload within 2 seconds in CI for
  configurations containing up to 100 projects, 100 variables, and 100 functions per project.
- **SC-006**: Every invalid configuration fixture reports at least one field-specific actionable
  error, retains the prior valid state, and exposes zero configured secret values.
- **SC-007**: All primary user journeys can be completed using only documented keyboard controls in
  Bash and Zsh terminals on both supported operating systems.
- **SC-008**: In 100% of secret-canary tests, secret values appear in none of logs, errors,
  diagnostics, crash reports, or user-visible shell-integration output. Intentional local disclosure
  after confirmed F2 warning and user-directed editing through F3 are explicitly excluded.
- **SC-009**: In an automated acceptance test that switches a project and then runs a non-switch
  command (e.g. `--help`) in the same shell session, the current-environment file is not
  re-sourced: no `shell-cmd` hook runs a second time, no shell function is redefined a second
  time, no environment variable changes, and the non-switch command's own output appears exactly
  once.
- **SC-010**: Against a fixture release server (never live GitHub), an automated test verifies:
  a newer stable release is installed and reported with the correct old/new version and path; an
  already-current installation changes nothing; and a missing/mismatched checksum, unsupported
  platform, or failed download each leave the previously installed executable byte-for-byte
  unchanged.

## Assumptions

- The initial release is single-user and local-only; synchronization between machines and shared
  multi-user configuration are outside scope.
- Bash and Zsh are the only supported shells for the initial release; other shells may receive an
  explanatory error but no integration changes.
- Project names are unique and case-sensitive.
- A project-specific definition overrides a shared definition with the same identifier.
- Selecting a project never changes the current working directory; `project` is informational
  metadata only, not a `cd` target, and its path need not exist.
- Shared definitions are recalculated on every successful switch rather than assumed to be already
  present in the shell.
- The starter configuration uses three illustrative projects and placeholder values; it never
  contains usable credentials.
- The user is responsible for deciding whether plaintext local environment values are acceptable and
  for using a dedicated secret manager when stronger protection is required.
- Remote configuration, automatic cloud authentication, credential rotation, and secret-manager
  integrations are outside the initial feature scope.
- Installation and profile modification always require an explicit user action; the product does not
  silently alter shell startup files on first run.
