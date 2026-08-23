# Phase 0 Research: Terminal Environment Switching

## Supported Go Toolchain

**Decision**: Use Go 1.26.x and pin the latest supported 1.26 patch in CI and releases. Review Go
1.27 adoption after a stabilizing patch and dependency compatibility validation.

**Rationale**: Go 1.27.0 was released on 2026-08-19, four days before this plan. Go supports the two
newest major lines, so 1.26 remains supported and has an established patch history.

**Alternatives considered**: Go 1.27.0 is current but very new; Go 1.25.x is mature but closer to
leaving the two-release support window.

Source: [Go release history](https://go.dev/doc/devel/release).

## TUI Stack

**Decision**: Use Bubble Tea v2, only needed Bubbles v2 components, and Lip Gloss v2. Pin mutually
compatible stable releases.

**Rationale**: Bubble Tea v2 is the maintained major line and provides explicit key messages,
terminal lifecycle handling, and deterministic model/update/view tests.

**Alternatives considered**: Bubble Tea v1 begins on the previous major; custom terminal control
substantially increases portability, cleanup, input-decoding, and accessibility risk.

Sources: [Bubble Tea releases](https://github.com/charmbracelet/bubbletea/releases),
[Bubbles v2 upgrade guide](https://github.com/charmbracelet/bubbles/blob/main/UPGRADE_GUIDE_V2.md).

## YAML Parsing and Validation

**Decision**: Use stable `gopkg.in/yaml.v3`, decode through `yaml.Node`, reject aliases, merge keys,
custom tags, duplicate keys, multiple documents, unknown fields, type mismatches, and trailing data;
then map into typed structures and run semantic validation.

**Rationale**: v3 is stable and provides source positions and `KnownFields`; node inspection covers
duplicate-key and unsupported-feature policy. The v4 line was still a release candidate.

**Alternatives considered**: YAML v4 was not stable; JSON conflicts with the required format; a
second YAML library adds unnecessary surface.

Sources: [go-yaml v3](https://github.com/go-yaml/yaml),
[go-yaml v4 status](https://pkg.go.dev/go.yaml.in/yaml/v4).

## Current-Shell Activation Protocol

**Decision**: The Bash/Zsh wrapper invokes the binary with the target shell and session-local lists
of managed names. The binary writes only a versioned payload to captured stdout and diagnostics to
stderr. The wrapper checks exit status and exact header/trailer markers before evaluation.

The payload snapshots the current directory and names it will change, removes obsolete names,
installs new definitions, changes directory, and updates ownership metadata. On activation failure,
it restores the pre-attempt state. This rollback does not preserve definitions overwritten by a
prior successful switch, matching the accepted clarification.

**Rationale**: Only code evaluated by the parent shell can apply `cd`, exports, functions, and
cleanup. Framing prevents errors or truncated output from being mistaken for activation code.

**Alternatives considered**: Unframed `eval "$(binary)"` may evaluate diagnostics; a persistent
activation file puts secrets on disk; wrapper-side YAML parsing duplicates logic in two shells.

## Shell Quoting and Function Trust Boundary

**Decision**: Generate literal assignments using audited Bash/Zsh-compatible single-quote encoding;
reject NUL bytes and restrict names to conservative ASCII identifiers. Never concatenate an
unquoted configured value into syntax.

Inline function bodies are trusted local code, not sanitizable data. Validate name, size, encoding,
target-shell syntax without execution, and file ownership/permissions. Warn that selecting a project
installs configured code on first run and whenever the deterministic function digest changes.
Persist acknowledgement as an atomic, user-only, digest-only record. This trust boundary must remain
visible in security review and documentation.

**Rationale**: Literal data can be quoted, but arbitrary shell programs cannot be made safe through
escaping. Required completion examples use real shell syntax.

**Alternatives considered**: An allowlisted DSL breaks general functions; external scripts only move
the trust boundary; generic parsers do not fully model interactive Zsh.

Reference: [mvdan shell syntax package](https://pkg.go.dev/mvdan.cc/sh/v3/syntax).

## Managed State Lifecycle

**Decision**: Keep newline-delimited identifier-only managed variable/function lists in private,
non-exported wrapper variables. Pass only names and target shell to the binary through environment
variables; never pass values or bodies in argv. A successful payload replaces these lists; failure
or cancellation leaves them unchanged.

**Rationale**: Cleanup requires prior state that is specific to one shell session. Global persistence
would create cross-session races.

**Alternatives considered**: A global state file conflicts across terminals; environment inference
cannot identify obsolete names/functions; restoring collisions was rejected by clarification.

## Trusted-Function Acknowledgement State

**Decision**: Store a schema version and SHA-256 digest of the canonical function-definition set in
a `0600` metadata file under `~/.env-switcher`. Write it using a same-directory temporary file,
`fsync`, atomic rename, and user-only directory permissions. Never store function bodies, variable
values, project paths, or secret-bearing diagnostics in this record.

**Rationale**: A persistent digest avoids repeated warnings for unchanged trusted code while keeping
the acknowledgement non-executable and non-secret. Atomic replacement prevents a partial record from
suppressing a required warning.

**Alternatives considered**: Storing bodies duplicates trusted code and increases disclosure risk;
process-only acknowledgement repeats every run; embedding acknowledgement in settings mutates the
user's source of truth.

## Cross-Shell Function Representation

**Decision**: A configured function accepts either one scalar body intended for both shells or a
map with explicit `bash` and/or `zsh` bodies. A missing target-specific body means that function is
not present in the effective environment for that shell.

**Rationale**: Shell functions such as completion setup commonly use incompatible Bash and Zsh
syntax. Requiring one body would reject useful configurations or fail at activation.

**Alternatives considered**: Requiring all bodies to be portable excludes shell-native features;
silently sending Zsh code to Bash is unsafe; separate settings files fragment the source of truth.

## Safe Installation and Rollback

**Decision**: Default to `~/.bashrc` for Bash and `~/.zshrc` for Zsh with explicit profile override;
refuse symlink targets by default. Under an exclusive lock, validate the profile, write a `0600`
timestamped backup and metadata, render one managed block, fsync a same-directory temporary file,
preserve mode, and atomically rename. Rollback atomically restores a verified backup.

**Rationale**: Same-filesystem rename prevents partial profiles. Backups and delimiters make updates
repeatable and reversible without touching unrelated content.

**Alternatives considered**: Repeated append duplicates blocks; whole-file replacement loses user
content; automatic symlink following creates surprising scope and atomicity risks.

## Test and Release Strategy

**Decision**: Combine unit/table/fuzz tests, golden contracts, real Bash/Zsh tests in temporary homes,
Linux/macOS CI, race/static checks, built-binary smoke tests, deterministic builds, and SHA-256
checksums.

**Rationale**: Fuzz/golden tests cover hostile quoting and rewriting boundaries; real shells prove
parent-shell effects; both operating systems are required for release confidence.

**Alternatives considered**: Unit-only tests miss shell behavior; container-only tests miss macOS.
