# Operations

## Cutting a release

Run the `Release` workflow from the Actions tab (`workflow_dispatch`, no tag push needed) and pick
a version-bump type (`patch` — default, `minor`, or `major`). It computes the next `vX.Y.Z` from
the latest existing tag, runs the test suite, creates and pushes that tag, cross-builds
`linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`, zips each binary, and publishes a
GitHub Release with the zips plus `SHA256SUMS`. `CI` (`.github/workflows/ci.yml`) runs the same
checks natively on Linux, Intel macOS, and Apple Silicon macOS on every push/PR.

## Verify a release

Download the platform `.zip` together with `SHA256SUMS`, verify, then unzip:

```bash
sha256sum --check SHA256SUMS      # Linux
shasum -a 256 --check SHA256SUMS  # macOS
unzip env-switcher_<os>_<arch>.zip
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
`install` again after upgrading the binary re-syncs the managed block to the latest wrapper. If an
executable from before the `bin/` convention existed (`~/.env-switcher/env-switcher`), it's removed
once the canonical `bin/env-switcher` copy is confirmed in place — never before, and never if it's
a symlink or owned by a different user.

## Upgrade

```bash
env-switcher upgrade
# or
env-switcher --upgrade
```

Checks the latest **stable** (non-draft, non-prerelease) release of
[oleksdovz/env-switcher](https://github.com/oleksdovz/env-switcher) by semantic-version comparison
against the running build, downloads the asset matching the running `GOOS`/`GOARCH`, verifies it
against the release's `SHA256SUMS`, and atomically replaces `~/.env-switcher/bin/env-switcher`. A
release with no published checksum, an unmatched checksum, or no asset for the running platform
fails the upgrade without touching the existing installation; an already-current installation
reports that and installs nothing. `F6` in the terminal interface (after a confirmation prompt)
runs the identical check — never a separate implementation.

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
environment and write it to `~/.env-switcher/current-env` (`0600`) — the only three actions that
ever do. The installed shell function clears that file *before* every invocation and `source`s it
afterward only if the invocation just rewrote it, so any other command (`--help`, `list`, `get`,
`version`, `validate`, `reload`, `view`, `edit`, `install`, `upgrade`, `rollback`, `uninstall`) —
and a failed switch attempt — never reactivates whatever an earlier, unrelated successful switch
left behind. See [security](security.md) for the failure-handling contract.
