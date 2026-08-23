# Contract: Installation, Rollback, and Uninstall

## Targets

- Executable: `~/.env-switcher/bin/env-switcher`.
- Bash default: `~/.bashrc`; Zsh default: `~/.zshrc`.
- Backups: `~/.env-switcher/backups/`.
- Activation file: `~/.env-switcher/current-env` (written by switching, not by installation).
- An explicit profile override must resolve to a user-owned file outside env-switcher data.

## Preconditions

- Shell is `bash` or `zsh`; the user confirms executable/profile targets, except for the silent
  already-installed self-update case described below.
- Profile is a user-owned regular file or absent; symlinks fail closed.
- Data and sensitive file permissions can be enforced.
- No concurrent install/rollback holds the exclusive installation lock.

## Managed Block

```text
# >>> env-switcher managed block v1 >>>
#env-switcher
env-switcher() {
  local __env_switcher_status
  __ENV_SWITCHER_TARGET_SHELL=<bash|zsh> "$HOME/.env-switcher/bin/env-switcher" "$@"
  __env_switcher_status=$?
  [ -f "$HOME/.env-switcher/current-env" ] && source "$HOME/.env-switcher/current-env"
  return $__env_switcher_status
}
# <<< env-switcher managed block v1 <<<
```

The wrapper forwards every argument, so `install`/`get`/`list`/`edit`/`help`/`version`, `--flag`
equivalents, and a bare project name all reach the executable the same way; only the exported
target-shell value differs between the Bash and Zsh templates.

Zero or one complete block is valid before reconciliation. Multiple, nested, reversed, or incomplete
markers fail closed; the installer never guesses a deletion range. The block is reconciled to the
current template either by explicitly running `install` again after a binary upgrade, or by the
self-install trigger below; the product never edits shell startup files as a side effect of any
other command (`list`, `get`, `validate`, ...).

## Self-Install Trigger

The default (no-argument) invocation compares its own resolved executable path against
`~/.env-switcher/bin/env-switcher` before doing anything else:

- **Paths match** (the ordinary case — running through the installed shell function): no
  self-install activity of any kind.
- **Paths differ, and nothing exists at the installed path yet** ("fresh"): print a confirmation
  naming exactly what will be created/changed (the data directory, starter settings file, detected
  shell's integration) and wait for `y`. Confirmed, this runs the full Install Transaction below
  plus `config.Bootstrap()`; declined, none of it runs and the requested action (the TUI) still
  does. An unsupported or undetected shell still creates the settings file but skips shell
  integration, with a note to run `install` manually.
- **Paths differ, and something exists at the installed path** ("already installed"): silently run
  the same Install Transaction, no confirmation. This is what lets "download a new build and just
  run it" keep both the executable and the managed block current without a separate `install` step.

## Install Transaction

1. Resolve paths without following unapproved symlinks and acquire the lock.
2. Validate current profile and executable target.
3. Create `0600` backups and metadata before changing existing targets.
4. Copy executable through same-directory temporary file, set mode, fsync, and atomic rename.
5. Reconcile exactly one block in memory, preserving every unrelated byte.
6. Write profile through same-directory temporary file, preserve mode, fsync, and atomic rename.
7. Verify executable and block; report backup identifiers.

Repeated identical installation does not change profile bytes or duplicate blocks. A no-op need not
create another backup.

## Rollback and Uninstall

Rollback selects a backup for the exact target, verifies owner, mode, canonical path, scope, and
SHA-256 digest, backs up current state, then atomically restores. Failure changes nothing.

Uninstall removes only one valid managed block and, after confirmation, the installed executable.
Settings/backups remain by default. Malformed markers fail closed. Both use the same backup and
atomic replacement guarantees.

Interruption before rename leaves the original target intact. Failure after executable replacement
but before profile replacement is reported with an exact rollback route; the unchanged profile does
not invoke an invalid new block. Tests always use temporary homes.
