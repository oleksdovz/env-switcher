# Operations

## Verify a release

Download the platform artifact together with `SHA256SUMS`, then verify before installation:

```bash
sha256sum --check SHA256SUMS   # Linux
shasum -a 256 --check SHA256SUMS  # macOS
```

Only use the checksum entry matching `linux|darwin` and `amd64|arm64`. Release artifacts are
self-contained and do not require root privileges.

## Install

Running the bare binary (`env-switcher`, no arguments) from anywhere — a fresh download, a build
in `dist/`, wherever — self-installs: on a machine with nothing at `~/.env-switcher/bin/env-switcher`
yet, it asks once before creating `~/.env-switcher`, writing the starter settings file, copying
itself to `~/.env-switcher/bin/`, and adding the managed shell function; on a machine that already
has that executable, it silently refreshes both without asking. Declining the first-run prompt, or
running with any other supported shell, leaves the environment untouched beyond what was requested.

The explicit form is still available and useful for a specific shell/profile override, or a
non-interactive install:

```bash
env-switcher install --shell zsh
# or
env-switcher install --shell bash
```

Installation copies the executable to `~/.env-switcher/bin/env-switcher` and atomically reconciles
one managed `env-switcher` shell function in `.zshrc` or `.bashrc`. Existing profiles are backed up
under `~/.env-switcher/backups/`. Symlink profiles and malformed markers fail closed. Running
`install` again after upgrading the binary re-syncs the managed block to the latest wrapper.

## Rollback and uninstall

```bash
env-switcher rollback --shell zsh
env-switcher uninstall --shell zsh
```

Rollback verifies the latest matching backup digest before restoration. Uninstall removes only the
managed block and installed executable; settings and backups remain.

Validate with `env-switcher validate`. If installation fails after copying the executable, the
unchanged profile will not invoke it; rerun installation or use rollback after correcting the
reported filesystem condition.

## Switching

`env-switcher <project>` (bare or via `--select`) and the TUI both resolve the effective
environment and write it to `~/.env-switcher/current-env` (`0600`); the installed shell function
then `source`s that file. See [security](security.md) for the failure-handling contract.
