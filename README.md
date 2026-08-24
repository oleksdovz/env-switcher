# env-switcher

`env-switcher` is a local Go CLI and keyboard-driven TUI for switching a project's environment
variables and trusted shell functions in the current Bash or Zsh process. The picker itself is a
[charm.land/huh](https://github.com/charmbracelet/huh) `Select` field, embedded in a small outer
Bubble Tea model that owns only the application-level shortcuts (view/edit/reload/install/
upgrade/quit). It supports Linux and macOS. Switching never changes the shell's current directory
— each project's `project:` path in `settings.yaml` is informational metadata only (shown by
`list`/`get`), not a `cd` target.

## Build and validate

```bash
go build -trimpath -o env-switcher ./cmd/env-switcher
go test ./...
go vet ./...
```

## Getting started

Just run the binary you built or downloaded, from wherever it is (`./env-switcher`,
`~/Downloads/env-switcher`, `dist/env-switcher`, ...) — with no arguments, so it opens the TUI:

- **First time on a machine** (nothing installed yet at `~/.env-switcher/bin/env-switcher`): it
  asks once, in the terminal, before doing anything —
  `First run: create ~/.env-switcher with a starter settings.yaml and install bash shell
  integration? [y/N]`. Confirming creates `~/.env-switcher/settings.yaml` (three example
  projects, non-functional placeholder credentials), copies the running binary to
  `~/.env-switcher/bin/env-switcher`, and adds the managed `env-switcher` shell function to
  `.bashrc`/`.zshrc` (detected from `$SHELL`; unsupported/undetected shells get the settings
  file but a note to run `install` manually). Declining just runs that one invocation without
  installing anything.
- **Already installed** (you downloaded a newer build and ran it directly): no prompt — it
  silently copies the new binary into `~/.env-switcher/bin/` and refreshes the managed shell
  block if its template changed, exactly like re-running `env-switcher install` would.
- **Running through the installed shell function** (the normal case, every day): none of the
  above happens at all — it just opens the TUI.

You can also do all of this explicitly at any time with `env-switcher install --shell bash` or
`--shell zsh`, `env-switcher rollback`, and `env-switcher uninstall`.

## Commands

```text
env-switcher                  open the interactive project picker
env-switcher <project>        switch directly to <project>
env-switcher list | ls        list configured projects
env-switcher get <project>    show one project's resolved configuration
env-switcher edit [project]   open settings.yaml in $VISUAL/$EDITOR
env-switcher validate         validate settings.yaml
env-switcher reload           re-validate settings.yaml non-interactively
env-switcher view             show the complete settings.yaml after confirmation
env-switcher install          install or update shell integration
env-switcher rollback         restore the previous shell profile backup
env-switcher uninstall        remove the managed shell integration
env-switcher upgrade          install the latest compatible release
env-switcher version          print build metadata
env-switcher help             show help
```

Every command above also accepts an equivalent `--flag` form (`--list`, `--get`, `--edit`,
`--validate`, `--reload`, `--view`, `--install`, `--rollback`, `--uninstall`, `--upgrade`,
`--version`, `--select`) so the installed shell function can forward `"$@"` regardless of style.

The TUI keys are `Enter` to select, `F2`/`v` to view, `F3`/`e` to edit, `F4`/`r` to reload,
`F5`/`i` to install, `F6` to upgrade, and `F10`/`q` to exit. Only `Enter` (and, at the CLI,
`env-switcher <project>`/`--select`) ever activate a project — every other command, including
`--help` and `upgrade`, is read-only with respect to the current shell (see "How switching works"
below).

## How switching works

Installation adds a managed block like this to `.bashrc`/`.zshrc`:

```bash
#env-switcher
env-switcher() {
  local __env_switcher_status __env_switcher_payload="$HOME/.env-switcher/current-env"
  rm -f "$__env_switcher_payload"
  __ENV_SWITCHER_TARGET_SHELL=bash "$HOME/.env-switcher/bin/env-switcher" "$@"
  __env_switcher_status=$?
  [ -f "$__env_switcher_payload" ] && source "$__env_switcher_payload"
  return $__env_switcher_status
}
```

Selecting a project (through the TUI, `env-switcher <project>`, or `env-switcher --select
[project]`) resolves the effective variables, shell functions, and shell-cmd hooks, then writes
them as a plain shell script to `~/.env-switcher/current-env` (mode `0600`). Those are the only
three actions that ever write it. The wrapper clears that file *before* every invocation and only
`source`s it afterward if the invocation just rewrote it — so a switch defines the functions and
exports the variables (never changing directory, never removing anything a previously active
project set — each switch only ever adds/overwrites the newly selected project's own definitions
on top of whatever the shell already has), while every other command (`--help`, `list`, `get`,
`version`, `validate`, `reload`, `view`, `edit`, `install`, `upgrade`, `rollback`, `uninstall`)
leaves the running shell exactly as it was, even if an earlier switch in that session left
something at `current-env`. There's no snapshot/rollback within a switch either: if one name
fails to apply (e.g. it's already `readonly` in your shell), the shell reports its own error for
that one statement and everything else in the file still applies. If a switch attempt fails to
resolve at all, nothing is written, so the wrapper has nothing to source and the shell is left
exactly as it already was.

## Configuration examples

Shell functions are a single body offered to whichever shell is active (no separate `bash`/`zsh`
form); a function needing genuinely different behavior per shell branches on `$ZSH_VERSION` at
runtime. `shell-cmd` is an anonymous hook (shared and/or per-project, both run if both are set)
that executes as the last step of every switch — for one-off setup that doesn't need to be a
named, callable function.

Every switch also sets `_PROJECT`, a reserved variable holding `project` resolved to a clean
absolute path (`~/`, `$HOME`, and `${HOME}` are expanded; nothing else is). Don't declare it
yourself — env-switcher always overwrites it — but do use it, quoted, in `env-vars` (as
`$_PROJECT`/`${_PROJECT}`, substituted the same way), `shell-functions`, and `shell-cmd`:

```yaml
shared:
  shell-functions:
    cd_project: |
      cd "$_PROJECT"

envs:
  dev:
    project: $HOME/projects/dev
    env-vars:
      CODEX_HOME: $_PROJECT/.codex
```

### Simple

Just projects and plain variables — no `shared:`, no functions, no merging.

```yaml
version: 1

envs:
  dev:
    project: ~/projects/dev
    env-vars:
      APP_ENV: development
      API_URL: http://localhost:3000/api

  staging:
    project: ~/projects/staging
    env-vars:
      APP_ENV: staging
      API_URL: https://staging.example.com/api
```

### More elaborate

`shared:` env-vars, shell-functions, and shell-cmd apply to **every** project automatically — no
merge key required for that. A project overrides a shared name just by defining the same name
itself (`LOG_LEVEL`, `AWS_REGION` for `prod` below); a project's own `shell-cmd` runs after the
shared one, not instead of it.

```yaml
version: 1

shared:
  env-vars:
    LOG_FORMAT: json
    AWS_REGION: eu-west-1
  shell-functions:
    whoami_env: |
      printf 'Project: %s\n' "${PROJECT_NAME:-unknown}"
  shell-cmd: |
    printf 'env-switcher: activated %s\n' "$__ENV_SWITCHER_ACTIVE_PROJECT"

envs:
  dev:
    project: ~/projects/dev
    env-vars:
      PROJECT_NAME: dev
      APP_ENV: development
      AWS_PROFILE: dev-profile
      LOG_LEVEL: debug
    shell-functions:
      deploy: |
        echo "deploying dev..."
    shell-cmd: |
      export DEV_TOOLS_ENABLED=true

  staging:
    project: ~/projects/staging
    env-vars:
      PROJECT_NAME: staging
      APP_ENV: staging
      AWS_PROFILE: staging-profile
      LOG_LEVEL: info

  prod:
    project: ~/projects/prod
    env-vars:
      PROJECT_NAME: prod
      APP_ENV: production
      AWS_PROFILE: prod-profile
      LOG_LEVEL: warn
      # Overrides the shared AWS_REGION for this project only
      AWS_REGION: us-east-1
```

### Simple, with a YAML merge key

YAML anchors (`&name`), aliases (`*name`), and merge keys (`<<: *name`) are also supported —
useful when you want the reuse spelled out explicitly right inside each project (or want more than
one reusable group, unlike the single global `shared:` above). An explicit key in the mapping
always wins over a merged one, so `staging` below still gets its own `AWS_REGION`.

```yaml
version: 1

shared:
  env-vars: &common_vars
    LOG_FORMAT: json
    AWS_REGION: eu-west-1

envs:
  dev:
    project: ~/projects/dev
    env-vars:
      <<: *common_vars
      APP_ENV: development
      LOG_LEVEL: debug

  staging:
    project: ~/projects/staging
    env-vars:
      <<: *common_vars
      APP_ENV: staging
      # Overrides the merged AWS_REGION for this project only
      AWS_REGION: us-west-2
```

Anchors have two safety limits: an anchored value can't itself reference another anchor, and the
document's total size after resolving every alias is capped independently of the raw file size —
see [settings-schema.md](specs/001-env-switching/contracts/settings-schema.md) for the exact
rules. The shipped starter file (created on first run) is a larger worked example combining all of
the above, plus Unicode values and other edge cases.

## Upgrading

```bash
env-switcher upgrade      # or: env-switcher --upgrade
```

Checks the latest **stable** (non-draft, non-prerelease) release of
[oleksdovz/env-switcher](https://github.com/oleksdovz/env-switcher) against the running build's
own version, using semantic-version comparison — not just "most recently published" — and reports
`env-switcher is already up to date (<version>)` if nothing newer is available. Otherwise it:

1. picks the release asset matching the running `GOOS`/`GOARCH` (failing with an actionable
   message, listing what the release *does* publish, if there's no build for your platform);
2. downloads it to a temporary file in `~/.env-switcher/bin/`;
3. verifies it against the release's published `SHA256SUMS` — a release with no checksum file is
   treated as a hard failure, never installed unverified;
4. if the asset is an archive, extracts only the single expected `env-switcher` executable,
   rejecting absolute paths, `..` traversal, symlinks, or any other archive entry;
5. atomically replaces `~/.env-switcher/bin/env-switcher`, then prints the old version, new
   version, and installed path.

The existing binary is left untouched if any of these steps fails. `F6` in the interactive picker
(with a confirmation dialog first) runs the exact same upgrade logic — see
`internal/upgrade` and `internal/app/upgrade.go` — it's never reimplemented in the TUI.

> `settings.yaml` and `current-env` are plaintext and not a secure secret store. Use short-lived
> credentials and a dedicated secret manager for sensitive environments.

See [configuration](docs/configuration.md), [operations](docs/operations.md), and
[security](docs/security.md).
