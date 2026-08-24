# Contract: Current-Environment Activation File

## Channels

- The CLI writes the activation script to a fixed file, `~/.env-switcher/current-env`
  (mode `0600`), instead of stdout. It never prints values or function bodies to stdout/stderr.
- The write is atomic (same-directory temp file, `fsync`, rename) and happens **only after** a
  fully successful project resolution, and **only** for a bare invocation, a project name, or
  `--select` — no other command ever writes it. Any failure (or any other command) leaves an
  existing file byte-for-byte unchanged.
- The installed shell function first removes `current-env` (`rm -f`), then invokes the CLI
  (inheriting the real stdout/stderr — there is no command-substitution capture step), then does
  `[ -f "$HOME/.env-switcher/current-env" ] && source "$HOME/.env-switcher/current-env"`.

Clearing the file first, and sourcing it only if the invocation just recreated it, is what
distinguishes "this run actually switched" from "a run once switched, at some point in the past."
Without that first step, current-env would persist across unrelated invocations, and *any*
successful command — `--help` included — would re-source whatever a previous switch left behind:
its variables re-exported, its shell-cmd hooks re-run, its functions redefined. A failed switch
attempt behaves the same way as any other non-writing command: nothing is there to source, so the
shell is left exactly as it was, not reactivated with the last successful state. This is what "no
change on failure" means, without a wrapper-side validation step and without relying on the CLI's
exit code (a non-switch command also exits 0, so exit code alone can't distinguish the two).

## Inputs

The wrapper exports `__ENV_SWITCHER_TARGET_SHELL` (`bash`/`zsh`, static per installed profile) and
forwards `"$@"` verbatim to the executable.

## Content

The file contains only shell statements — no envelope, header, or transaction id, and no
snapshot/rollback machinery. It is safe to `cat` (it may contain quoted secret values, so treat it
like `settings.yaml`) and is regenerated in full on every successful switch, in this order:

1. define the project's effective functions;
2. assign/export the project's effective variables;
3. export `__ENV_SWITCHER_ACTIVE_PROJECT` so it's visible for the rest of the session;
4. run shell-cmd hooks (shared, then project).

There is no directory-change step: `project` in settings.yaml is informational metadata (shown by
`list`/`get`) and switching never runs `cd` or checks that the path exists. There is also no
managed-name cleanup: a switch only ever applies the newly selected project's own definitions on
top of whatever the shell already has — nothing from a previously active project (or from outside
env-switcher entirely) is ever removed.

This is a plain sequential script, not a transaction: any individual statement can fail (an export
refused because the name is `readonly`, for instance) without a guard clause around it. A failure
prints the shell's own error and execution continues to the next statement in the same file — a
partial application within one switch is possible, by design, in exchange for a much shorter file.

## Quoting and Trust

- Names follow the conservative ASCII identifier grammar and cannot collide with a reserved CLI
  command word.
- Values/paths use single-quote encoding; embedded quotes close, escape, and reopen the literal.
- NUL is rejected; no configured literal is emitted unquoted or expanded.
- Function bodies are trusted code emitted only after permission, size, encoding, name, and
  target-shell syntax checks.
- YAML cannot influence the generated script or the fixed file path.

## Wrapper Responsibilities

- Invoke the fixed installed executable (`~/.env-switcher/bin/env-switcher`) through a shell
  function named `env-switcher`, forwarding all arguments and exporting the target shell.
- Clear `current-env` before invoking the executable, so its presence afterward reflects only
  this invocation.
- Preserve and return the CLI's own exit status (captured before sourcing current-env, since
  `source` overwrites `$?`).
- Source `current-env` only when present *after* the invocation; do nothing when absent — which
  covers both "this command doesn't switch" and "this switch attempt failed."
- Never print the file's content because it may contain secrets.

Contract changes to the generated script require fixtures for both shells; the wrapper itself has
no version negotiation to break, since it does not parse the file it sources.
