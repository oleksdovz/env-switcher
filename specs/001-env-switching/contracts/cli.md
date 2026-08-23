# Contract: CLI and TUI

## Commands

Every row accepts the bare word shown; where a `--flag` alias is listed it is fully equivalent, so
the installed shell function can forward `"$@"` regardless of which style was typed.

| Command | Alias | Purpose | Output contract |
|---------|-------|---------|-----------------|
| `env-switcher` | | Self-install (see [installation.md](installation.md)) if needed, then open the TUI | Renders on stdout/stderr like any terminal program; a selection writes `current-env` (see [shell-payload.md](shell-payload.md)) |
| `env-switcher <project>` | `--select <project>` | Switch directly, no TUI | Writes `current-env` on success; non-zero exit and no change to `current-env` on failure |
| `env-switcher list` | `--list`, `ls` | List configured projects | Names and directories on stdout; no secrets, no warning |
| `env-switcher get <project>` | `--get <project>` | Show one project's resolved configuration | stderr advisory, then unmasked values/bodies on stdout; no blocking prompt (stays script-safe) |
| `env-switcher edit [project]` | `--edit [project]` | Open settings in the resolved editor | Opens the whole file; an unknown `project` is a non-fatal stderr note |
| `env-switcher validate` | | Validate settings | Diagnostics only |
| `env-switcher reload` | | Validate settings in a non-TUI invocation | Diagnostics only; TUI F4 additionally replaces its in-memory model |
| `env-switcher view` | | Show the complete settings file after confirmation | Same warning as TUI F2 |
| `env-switcher install` | | Install/update integration | Confirmation required unless explicit approval flag |
| `env-switcher rollback` | | Restore verified backup | Selection and confirmation required |
| `env-switcher uninstall` | | Remove managed integration | Preserve settings/backups by default |
| `env-switcher version` | `--version` | Show build metadata | Human stdout |
| `env-switcher help` | `--help`, `-h` | Show usage | Human stdout |

A bare word matching none of the above is treated as `<project>`. `config.Validate` rejects
project names that collide with a command word, so this is never ambiguous.

## Outcome Classes

Stable classes are: success (`0`), operation (`1`), cancelled (`2`), configuration (`3`),
compatibility (`4`), and security (`5`). These numeric codes are frozen by contract tests. Errors
contain context but never configured values, function bodies, or activation-file text. The wrapper
itself never inspects or rejects a payload — it only preserves the CLI's exit code (captured before
`source`, since sourcing overwrites `$?`) and conditionally sources `current-env`.

The target shell reaches the CLI as an exported environment variable
(`__ENV_SWITCHER_TARGET_SHELL`), and prior managed-name ownership reaches it as ordinary inherited
process environment (see [shell-payload.md](shell-payload.md)). Environment values, function
bodies, and the generated activation script never appear in process arguments.

## TUI Keys

| Key | Alternative | Action |
|-----|-------------|--------|
| arrows | `j` / `k` | Move focus |
| `Enter` | explicit select subcommand | Confirm selection |
| `F2` | `v` | Warn about sensitive values, then show complete unmasked settings after confirmation |
| `F3` | `e` | Open settings in editor |
| `F4` | `r` | Validate/reload atomically |
| `F5` | `i` | Confirmed install/update |
| `F10` | `q` / `Esc` | Exit without unconfirmed activation |

The footer displays actions. Small terminals use a condensed/scrollable view. F3 suspends and
restores terminal mode. Editor resolution is `VISUAL`, then `EDITOR`; parse command/arguments without
a shell and append settings path as one literal argument.

F2 is an intentional local secret-disclosure path: before showing the complete unmasked file, the
TUI warns that it contains sensitive values and requires continuation. Cancellation shows nothing.
The displayed content is never copied into logs, errors, diagnostics, crash reports, or integration
diagnostics. User-directed F3 editing is the second intentional local disclosure path.

On first run and whenever the deterministic shell-function digest changes, the TUI warns that
configured functions are trusted executable user code. F2, F3, validation, and reload may read or
compare function bodies but never execute them; execution is permitted only after project selection.

F4 replaces the model only with a fully valid candidate. On failure, prior list/focus remains. If a
focused project disappears after success, focus moves to the first sorted project without activation.
