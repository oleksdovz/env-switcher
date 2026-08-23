# env-switcher

`env-switcher` is a local Go CLI and keyboard-driven Bubble Tea TUI for switching a project's
environment variables and trusted shell functions in the current Bash or Zsh process. It supports
Linux and macOS. Switching never changes the shell's current directory — each project's `project:`
path in `settings.yaml` is informational metadata only (shown by `list`/`get`), not a `cd` target.

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
env-switcher version          print build metadata
env-switcher help             show help
```

Every command above also accepts an equivalent `--flag` form (`--list`, `--get`, `--edit`,
`--version`, `--select`) so the installed shell function can forward `"$@"` regardless of style.

The TUI keys are `Enter` to select, `F2`/`v` to view, `F3`/`e` to edit, `F4`/`r` to reload,
`F5`/`i` to install, and `F10`/`q` to exit.

## How switching works

Installation adds a managed block like this to `.bashrc`/`.zshrc`:

```bash
#env-switcher
env-switcher() {
  local __env_switcher_status
  __ENV_SWITCHER_TARGET_SHELL=bash "$HOME/.env-switcher/bin/env-switcher" "$@"
  __env_switcher_status=$?
  [ -f "$HOME/.env-switcher/current-env" ] && source "$HOME/.env-switcher/current-env"
  return $__env_switcher_status
}
```

Selecting a project (through the TUI or `env-switcher <project>`) resolves the effective variables,
shell functions, and shell-cmd hooks, then writes them as a plain shell script to
`~/.env-switcher/current-env` (mode `0600`). The wrapper function immediately `source`s that file,
which defines the functions and exports the variables — it never changes directory, and it never
removes anything a previously active project set; each switch only ever adds/overwrites the newly
selected project's own definitions on top of whatever the shell already has. There's no
snapshot/rollback either: if one name fails to apply (e.g. it's already `readonly` in your shell),
the shell reports its own error for that one statement and everything else in the file still
applies. If a switch fails to resolve at all, `current-env` is left untouched, so sourcing it again
just reapplies the last successful state.

## Configuration examples

Shell functions are a single body offered to whichever shell is active (no separate `bash`/`zsh`
form); a function needing genuinely different behavior per shell branches on `$ZSH_VERSION` at
runtime. `shell-cmd` is an anonymous hook (shared and/or per-project, both run if both are set)
that executes as the last step of every switch — for one-off setup that doesn't need to be a
named, callable function.

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

> `settings.yaml` and `current-env` are plaintext and not a secure secret store. Use short-lived
> credentials and a dedicated secret manager for sensitive environments.

See [configuration](docs/configuration.md), [operations](docs/operations.md), and
[security](docs/security.md).
