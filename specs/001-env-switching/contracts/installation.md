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
  local __env_switcher_status __env_switcher_payload="$HOME/.env-switcher/current-env"
  rm -f "$__env_switcher_payload"
  __ENV_SWITCHER_TARGET_SHELL=<bash|zsh> "$HOME/.env-switcher/bin/env-switcher" "$@"
  __env_switcher_status=$?
  [ -f "$__env_switcher_payload" ] && source "$__env_switcher_payload"
  _env_switcher_register_completion
  return $__env_switcher_status
}

_env_switcher_completion() { ... }          # see "Tab Completion" below
_env_switcher_register_completion() { ... } # complete -F ... (Bash) / compdef ... (Zsh)
_env_switcher_register_completion
# <<< env-switcher managed block v1 <<<
```

The wrapper forwards every argument, so `install`/`get`/`list`/`edit`/`help`/`version`/`upgrade`,
`--flag` equivalents, and a bare project name all reach the executable the same way; only the
exported target-shell value differs between the Bash and Zsh templates. Clearing the payload file
before invoking, and sourcing it afterward only if present, is what keeps every command besides an
actual switch (bare invocation, project name, `--select`) from reactivating whatever an earlier,
unrelated successful switch left there — see [shell-payload.md](shell-payload.md).

Zero or one complete block is valid before reconciliation. Multiple, nested, reversed, or incomplete
markers fail closed; the installer never guesses a deletion range. The block is reconciled to the
current template either by explicitly running `install` again after a binary upgrade, or by the
self-install trigger below; the product never edits shell startup files as a side effect of any
other command (`list`, `get`, `validate`, ...). A **fresh** install (no existing markers) inserts
the block 5 lines before the end of the profile rather than strictly appending at the very end —
if the profile has 5 or fewer lines, the block goes at the very beginning instead — so content the
user deliberately put last (a prompt theme finalization, another tool's `eval "$(... init)"`, ...)
keeps running after this block, not before it. Reconciling an *existing* block never moves it; it's
rewritten exactly where the markers already were.

## Tab Completion

`_env_switcher_completion` (Bash: registered via `complete -F`; Zsh: via `compdef`, calling
`compadd --`) reads project names fresh on every completion attempt — from `settings.yaml` via
`yq -r '.envs | keys | .[]'` if `yq` is installed, falling back to the installed `env-switcher`
executable's own `list` output (invoked directly, never through the `env-switcher()` function
above) only once `settings.yaml` already exists, so a bare TAB press can never create one. Missing
`yq`, a missing executable, a missing/empty/invalid `settings.yaml` — any combination — degrades to
no candidates and no error, never a broken prompt or a stray `-`-prefixed candidate.

It's defined at the top level of the profile, not nested inside `env-switcher()`: a function
defined only inside another function doesn't exist to `complete`/`compdef` until that outer
function has actually run once, so nesting it would mean completion only starts working after the
first invocation in a session. `_env_switcher_register_completion` re-runs the registration itself
at the end of every `env-switcher()` call, not only once at shell startup, because — in Zsh
specifically — a configured `shell-cmd` that calls `compinit` again (a common way to reload some
other tool's completions right after a switch) wipes Zsh's entire completion registry, silently
losing this binding along with everything else; re-registering after every invocation is cheap (no
process spawned) and self-heals regardless of what caused the loss.

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

An executable left over at the pre-`bin/`-convention location (`~/.env-switcher/env-switcher`,
before this executable target existed) is removed once the canonical `bin/env-switcher` copy from
this transaction is confirmed in place — never before, and never if it's a symlink or not owned by
the current user. This runs after `install`, after the silent already-installed self-update, and
after a successful `upgrade`.

## Rollback and Uninstall

Rollback selects a backup for the exact target, verifies owner, mode, canonical path, scope, and
SHA-256 digest, backs up current state, then atomically restores. Failure changes nothing.

Uninstall removes only one valid managed block and, after confirmation, the installed executable.
Settings/backups remain by default. Malformed markers fail closed. Both use the same backup and
atomic replacement guarantees.

Interruption before rename leaves the original target intact. Failure after executable replacement
but before profile replacement is reported with an exact rollback route; the unchanged profile does
not invoke an invalid new block. Tests always use temporary homes.
