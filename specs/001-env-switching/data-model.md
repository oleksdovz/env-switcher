# Data Model: Terminal Environment Switching

## Model Boundaries

The model has three boundaries: persisted `settings.yaml`, validated immutable application data,
and session-local ownership state maintained by the Bash/Zsh wrapper. No secret or function body is
persisted outside settings. Profile backups are protected as sensitive files.

## Settings Document

| Field | Type | Required | Rules |
|-------|------|----------|-------|
| `version` | positive integer | yes | Initial schema is `1`; unsupported versions fail closed |
| `shared` | Shared Environment | no | Defaults to empty |
| `envs` | map of name to Project Environment | yes | 1-100 unique case-sensitive names |

The document is one UTF-8 YAML document, at most 1 MiB. Unknown fields, duplicate keys, custom
tags, non-string map keys, and additional documents are rejected. Anchors (`&name`), aliases
(`*name`), and merge keys (`<<: *name`, standard explicit-key-wins-over-merged override semantics)
are accepted, subject to two anchor-safety rules: an anchored value cannot itself reference another
anchor (keeps expansion to one hop, never chained), and the total size of the document after every
alias is resolved to its target must not exceed 8 MiB, independent of the 1 MiB raw-file cap. Source
line and column are retained for diagnostics. The entire document validates before use.

## Shared Environment

| Field | Type | Required | Rules |
|-------|------|----------|-------|
| `env-vars` | map of identifier to string | no | 0-100 unique names |
| `shell-functions` | map of identifier to Shell Function Definition body | no | 0-100 unique names; trusted local code |
| `shell-cmd` | Shell Command Hook body | no | Trusted local code; runs before the project's own |

Shared definitions enter every effective project. A same-kind project definition replaces the
shared definition with the same name. A variable and function may not share a name. `shell-cmd` is
additive across scopes, not replaced (see Shell Command Hook below).

## Project Environment

| Field | Type | Required | Rules |
|-------|------|----------|-------|
| project map key | string | yes | 1-64 characters; unique, case-sensitive, not a reserved CLI command word |
| `project` | path string | yes | Non-empty; `~` allowed only as first path segment |
| `env-vars` | map of identifier to string | no | 0-100 unique names |
| `shell-functions` | map of identifier to Shell Function Definition body | no | 0-100 unique names; trusted local code |
| `shell-cmd` | Shell Command Hook body | no | Trusted local code; runs after the shared one |

`project` is informational metadata only — shown by `list`/`get` — not a switching target.
Switching never runs `cd` and never checks whether the path exists or is a directory.

## Environment Variable Definition

| Field | Type | Rules |
|-------|------|-------|
| name | string | `[A-Za-z_][A-Za-z0-9_]*`; reserved internal prefix rejected |
| value | string | UTF-8; empty allowed; NUL prohibited; maximum 64 KiB |
| source | enum | `shared` or `project`; derived |

Values are opaque literals: no `$`, command, glob, escape, or tilde expansion. Exact case-sensitive
project names override shared names.

## Shell Function Definition

| Field | Type | Rules |
|-------|------|-------|
| name | string | Same identifier rule; reserved internal prefix rejected |
| body | string | One body, offered to whichever shell is active; UTF-8 trusted code; non-empty; NUL prohibited; maximum 256 KiB |
| source | enum | `shared` or `project`; derived |

There is no per-shell body split: one body is syntax-checked against, and offered to, whichever
shell is active at switch time. A function that genuinely needs different behavior per shell
handles that itself at runtime (e.g. branching on `$ZSH_VERSION`), rather than the schema carrying
two bodies. Settings must be current-user owned and not group/other writable before a body is
emitted. Syntax validity does not imply safety.

## Shell Command Hook (`shell-cmd`)

| Field | Type | Rules |
|-------|------|-------|
| body | string | Same rules as a Shell Function Definition's body, but anonymous — not callable on demand |
| scope | enum | `shared` or `project`; both may be set |

Unlike a named function, `shell-cmd` has no identifier. On a successful switch, the shared
`shell-cmd` (if set) runs first, then the project's own (if set) — both, never one replacing the
other — as the last statements after functions are defined and variables exported. It is exactly
as trusted as a named function and is included in the first-run/changed-function digest and
warning.

## Effective Environment

Immutable derived data containing selected name, sorted effective variables, sorted effective
functions, ordered shell-cmd hooks, target shell, and deterministic configuration digest. It does
not carry a directory — switching does not resolve or use one.

Derivation order:

1. validate the complete settings document;
2. copy shared variables/functions;
3. overlay project definitions by exact name;
4. reject cross-kind and reserved-name collisions;
5. validate target-shell function syntax;
6. sort operations deterministically.

## Shell Session State

| Field | Type | Persistence | Rules |
|-------|------|-------------|-------|
| active project | string | current shell only, exported | Empty before first successful switch |
| acknowledged function digest | string | user-only metadata file | Records the last warned trusted-code set across runs |

The active-project name uses a reserved prefix. There is no tracked list of previously managed
variable/function names: switching does not remove anything a prior switch (or anything else) set,
it only ever applies the newly selected project's own variables/functions/shell-cmd.

The settings model includes a deterministic digest of all function names, target-shell variants,
and bodies. Missing acknowledgement on first run or a changed digest triggers a trusted-code warning;
calculating or comparing the digest never executes a body. Persisted acknowledgement contains only
the digest and schema version, uses `0600` permissions under the `0700` data directory, verifies
current-user ownership, and is updated by same-directory temporary file, `fsync`, and atomic rename
only after consent. Missing, malformed, unsafe-permission, or mismatched metadata is treated as not
acknowledged and never suppresses the required warning.

```text
Uninitialized --successful activation--> Active(project A)
Active A     --successful activation--> Active(project B), A's names untouched
```

On successful A-to-B switching, B's variables/functions are applied on top of whatever the shell
already has; nothing from A (or from outside env-switcher) is removed. B overwrites any same-name
current definition, including one not created by env-switcher; originals are not saved or restored.

## Current Environment File

| Field | Type | Rules |
|-------|------|-------|
| path | fixed path | `~/.env-switcher/current-env`, mode `0600` |
| target shell | enum | Determined by the invoking wrapper (`bash` or `zsh`) |
| operations | ordered statements | Generated; configured literals always quoted |
| write condition | atomic, success-only | Written via same-directory temp file + rename only after
  a fully successful resolution; left untouched on any failure |

Operation order: define functions; assign/export variables; export the active-project name; run
shell-cmd hooks. There is no directory-change step (`project` is informational only) and no
snapshot/rollback: this is a plain sequential script, not a transaction. If one statement fails
(e.g. a shell `readonly` conflict on one variable), the shell reports its own error for that
statement and every other statement in the file still runs — a partial application is possible.
The installed shell function sources this file unconditionally after every invocation; because it
is only ever rewritten when resolution fully succeeds, sourcing a stale copy after a failed switch
reproduces the unchanged prior state rather than a half-written one.

## Installation Record and Backup

The managed block contains fixed markers and a minimal wrapper that forwards arguments to the
fixed executable path, but no configured values or function bodies.

| Backup field | Type | Rules |
|--------------|------|-------|
| backup id | timestamp plus suffix | Unique in backup directory |
| source path | absolute path | Exact approved profile target |
| backup path | absolute path | Under `~/.env-switcher/backups` |
| source mode | permission bits | Restored on rollback |
| digest | SHA-256 | Verified before rollback |
| created time | UTC timestamp | Informational and sortable |

Backup files/metadata use `0600`; the directory uses `0700`. Rollback validates ownership, scope,
and digest before same-directory atomic replacement.
