# env-switcher

`env-switcher` is a local Go CLI and keyboard-driven TUI for switching a project's environment
variables and trusted shell functions in the current Bash or Zsh process. The picker itself is a
[charm.land/huh](https://github.com/charmbracelet/huh) `Select` field, embedded in a small outer
Bubble Tea model that owns only the application-level shortcuts (view/edit/reload/install/
upgrade/quit). It supports Linux and macOS. Switching never changes the shell's current directory
— each project's `project:` path in `settings.yaml` is informational metadata only (shown by
`list`/`get`), not a `cd` target.

## Contents

- [Build and validate](#build-and-validate)
- [Getting started](#getting-started)
- [Commands](#commands)
- [How switching works](#how-switching-works)
- [Shell completion](#shell-completion)
- [Bundled environment variables](#bundled-environment-variables)
- [Configuration examples](#configuration-examples)
- [Upgrading](#upgrading)

## Build and validate

`make check` runs everything CI does, in order (format check, vet, test, race test, then build to
`dist/env-switcher`):

```bash
make check
```

Or individually:

| Target | Equivalent | What it does |
|--------|------------|---------------|
| `make fmt` | `test -z "$(gofmt -l .)"` | Fails if any file isn't `gofmt`-formatted |
| `make vet` | `go vet ./...` | Static analysis |
| `make test` | `go test ./...` | Unit and integration tests |
| `make race` | `go test -race ./...` | Same tests, with the race detector |
| `make build` | `go build -trimpath -o dist/env-switcher ./cmd/env-switcher` | Builds to `dist/env-switcher` |

Without `make`, run the right-hand column's `go` commands directly — `dist/` isn't required to
exist first; `go build -trimpath -o env-switcher ./cmd/env-switcher` (no `dist/` prefix) works
just as well if you'd rather have the binary in the repo root.

## Getting started

1. Build (or download) the binary, then just run it — with no arguments, so it opens the TUI:

   ```console
   $ make build
   $ ./dist/env-switcher
   First run: create ~/.env-switcher with a starter settings.yaml and install zsh shell integration? [y/N] y
   created ~/.env-switcher, installed env-switcher at ~/.env-switcher/bin/env-switcher, and added shell integration to ~/.zshrc
   restart your shell or run `source ~/.zshrc` to start using env-switcher
   ```

2. Reload your shell (`source ~/.zshrc`, or open a new terminal), then run it again — this time
   through the installed `env-switcher` shell function, with the starter config's three example
   projects (non-functional placeholder credentials) ready to pick from:

   ```console
   $ source ~/.zshrc
   $ env-switcher
   ```

   Pick one with `Enter` (arrow keys / `j`/`k` to move); the interface closes and the project's
   variables and functions are now live in this shell. `env-switcher list` (or tab-completion,
   `env-switcher <TAB>`) shows what's configured without opening the picker.

3. Edit `~/.env-switcher/settings.yaml` to replace the starter projects with your own —
   `env-switcher edit` opens it in `$VISUAL`/`$EDITOR` — following the
   [configuration examples](#configuration-examples) below.

What actually happens on that first bare run depends on the machine's state:

| Situation | Result |
|-----------|--------|
| Nothing installed yet at `~/.env-switcher/bin/env-switcher` | Asks once (shown above) before creating anything; declining runs that one invocation only, installing nothing |
| Something already installed there, but you ran a *different* copy (e.g. a freshly downloaded update) | No prompt — silently refreshes the installed binary and the shell block if its template changed, exactly like `env-switcher install` |
| Running through the already-installed shell function (the normal day-to-day case) | Neither of the above — it just opens the TUI |

You can also do all of this explicitly at any time: `env-switcher install --shell bash` (or
`--shell zsh`), `env-switcher rollback`, `env-switcher uninstall`.

## Commands

| Tool | Command | Argument | What it does |
|------|---------|----------|---------------|
| `env-switcher` | *(none)* | | Open the interactive project picker |
| `env-switcher` | *(none)* / `--select` | `<project>` | Switch directly to `<project>`, no TUI |
| `env-switcher` | `list` / `ls` / `--list` | | List configured projects |
| `env-switcher` | `get` / `--get` | `<project>` | Show one project's resolved configuration |
| `env-switcher` | `edit` / `--edit` | `[project]` | Open `settings.yaml` in `$VISUAL`/`$EDITOR` |
| `env-switcher` | `validate` / `--validate` | | Validate `settings.yaml` |
| `env-switcher` | `reload` / `--reload` | | Re-validate `settings.yaml` non-interactively |
| `env-switcher` | `view` / `--view` | | Show the complete `settings.yaml`, after confirmation |
| `env-switcher` | `install` / `--install` | | Install or update shell integration |
| `env-switcher` | `rollback` / `--rollback` | | Restore the previous shell profile backup |
| `env-switcher` | `uninstall` / `--uninstall` | | Remove the managed shell integration |
| `env-switcher` | `upgrade` / `--upgrade` | | Install the latest compatible release |
| `env-switcher` | `version` / `--version` | | Print build metadata |
| `env-switcher` | `help` / `--help` | | Show usage |

`<project>` is required, `[project]` optional. Every `--flag` form is fully equivalent to its
bare-word command (with the same argument, where one applies), so the installed shell function can
forward `"$@"` regardless of which style was typed — e.g. `env-switcher get dev` and
`env-switcher --get dev` do exactly the same thing.

Only `Enter` in the TUI, and `env-switcher <project>`/`--select` at the CLI, ever activate a
project — every other command, including `--help` and `upgrade`, is read-only with respect to the
current shell (see [How switching works](#how-switching-works)).

| TUI key | Alternative | Action |
|---------|-------------|--------|
| `Enter` | | Confirm the focused selection |
| `F2` | `v` | View the complete settings file, after confirmation |
| `F3` | `e` | Edit settings in `$VISUAL`/`$EDITOR` |
| `F4` | `r` | Reload/re-validate |
| `F5` | `i` | Install or update shell integration |
| `F6` | | Upgrade `env-switcher` |
| `F10` | `q` | Exit without activating anything |

`install`, `rollback`, `uninstall`, and `upgrade` show what they're about to do (shell, profile,
executable path; current/latest version) and ask for confirmation before changing anything — pass
`--yes` (`-y` also works for `upgrade`) to skip the prompt for scripted use. Switching to a name
that isn't configured lists what actually is, instead of a bare "not configured":

```text
⚠ project "stagng" is not configured

Configured projects:
  - dev
  - prod
  - staging

Run `env-switcher list` (or `ls`) to see this list again.
```

## How switching works

Installation adds a managed block (between `# >>> env-switcher managed block v1 >>>` and a
matching `<<<` line) to `.bashrc`/`.zshrc`. A fresh install doesn't just append it at the very end
of the file: it's inserted a few lines *before* the end, so anything the file's own last few
lines do (a prompt theme finalization, a final tool's `eval "$(... init)"`, ...) keeps running
after this block, not before it. Re-running `install` afterward always updates the block exactly
where it already is.

The block itself — shown here in full, since it's genuinely everything that gets added, not an
excerpt — defines the `env-switcher` function and its tab-completion (see
[Shell completion](#shell-completion)):

```bash
#env-switcher
env-switcher() {
  local __env_switcher_status __env_switcher_payload="$HOME/.env-switcher/current-env"
  rm -f "$__env_switcher_payload"
  __ENV_SWITCHER_TARGET_SHELL=bash "$HOME/.env-switcher/bin/env-switcher" "$@"
  __env_switcher_status=$?
  [ -f "$__env_switcher_payload" ] && source "$__env_switcher_payload"
  _env_switcher_register_completion
  return $__env_switcher_status
}

_env_switcher_completion() {
  local cur envs
  if command -v yq >/dev/null 2>&1 && [ -r "$HOME/.env-switcher/settings.yaml" ]; then
    envs=$(yq -r '.envs | keys | .[]' "$HOME/.env-switcher/settings.yaml" 2>/dev/null)
  fi
  if [ -z "$envs" ] && [ -r "$HOME/.env-switcher/settings.yaml" ] && [ -x "$HOME/.env-switcher/bin/env-switcher" ]; then
    envs=$("$HOME/.env-switcher/bin/env-switcher" list 2>/dev/null | cut -f1)
  fi
  [ -n "$envs" ] || return 0
  cur=${COMP_WORDS[COMP_CWORD]}
  COMPREPLY=($(compgen -W "$envs" -- "$cur"))
}

_env_switcher_register_completion() {
  complete -F _env_switcher_completion env-switcher
}

_env_switcher_register_completion
```

(Zsh's is the same shape, using `compdef`/`compadd` in place of `complete`/`COMPREPLY` — see
[Shell completion](#shell-completion) for the full template and why it's laid out this way.)

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

## Shell completion

Tab-completing a project name (`env-switcher <TAB>`) is part of the same managed block install
adds — there's no separate command or flag to turn it on. It's implemented entirely in the shell
template (Bash's `complete -F` / Zsh's `compdef`+`compadd`), not in the Go binary: **there is no
"generate completions" subcommand**. Every completion attempt reads the current environment names
fresh:

1. **Preferred**: [`yq`](https://github.com/mikefarah/yq) v4 reads `settings.yaml` directly —
   `yq -r '.envs | keys | .[]' ~/.env-switcher/settings.yaml` — no other process involved.
2. **Fallback**, only if `yq` isn't installed (or produced nothing usable): the installed
   `env-switcher` executable itself, invoked directly as `env-switcher list` — never through the
   `env-switcher()` wrapper function, so this can't source `current-env`, activate a project, or
   run any configured `shell-cmd`/`shell-function`; `list` is already a plain, read-only,
   non-switching command on its own. This fallback only runs once `settings.yaml` already exists,
   specifically so a bare TAB press can never have the side effect of creating one (`list`, like
   most commands, auto-bootstraps a starter file the first time anything runs on a machine with
   none yet — appropriate for a command the user typed, not for a completion attempt).

Every one of these degrades to **no candidates and no error**, never a broken prompt: `yq` not
installed, `env-switcher` not installed, no `settings.yaml` yet, an empty `envs:` map, or invalid
YAML. Completion never shells out to anything beyond `yq`/the `env-switcher` binary itself, and
project names starting with `-` are handled correctly (`compadd --` / `compgen -- "$cur"`) rather
than being mistaken for options.

Two more details worth knowing if you're reading the generated block itself (shown in full above):

- **The completion function is defined at the top level of the profile, not nested inside
  `env-switcher()`.** A function defined only inside another function doesn't exist yet — to Bash's
  `complete` or Zsh's `compdef` — until that outer function has actually run once. Nesting it would
  mean completion only starts working *after* the first `env-switcher` invocation in a session,
  which is exactly the bug this avoids.
- **The registration (`complete -F ...` / `compdef ...`) is re-asserted at the end of every
  `env-switcher()` call, not just once at shell startup.** In Zsh specifically, a configured
  `shell-cmd` that calls `compinit` again — a common way to (re)load some other tool's completions
  right after a switch — wipes Zsh's *entire* completion registry, not just entries added since the
  last `compinit`, silently losing this binding along with everything else. Re-registering after
  every invocation is cheap (no process spawned) and self-heals regardless of what caused the loss,
  so completion doesn't stop working after being used a second time.

## Bundled environment variables

Besides whatever your own `env-vars` define, every switch exports a couple of variables
env-switcher manages itself:

| Variable | Set by | Value | Reserved? |
|----------|--------|-------|-----------|
| `_PROJECT` | Every switch | `envs.<name>.project`, expanded (`~`, `$HOME`, `${HOME}`) and resolved to a clean absolute path | Soft — declaring it yourself in `env-vars` is accepted (existing configs keep loading), but env-switcher always overwrites it at switch time; never an error |
| `__ENV_SWITCHER_ACTIVE_PROJECT` | Every switch | The name of the project just activated (e.g. `dev`) | Hard — the entire `__ENV_SWITCHER_` prefix is rejected by `validate` for any `env-vars`/`shell-functions` name |
| `__ENV_SWITCHER_TARGET_SHELL` | The installed shell function, before it calls the binary | `bash` or `zsh` | Hard — internal wrapper→binary plumbing; never exported into your shell, nothing to reference from your own config |

- **`_PROJECT`** is available, quoted, wherever your config runs: as `$_PROJECT`/`${_PROJECT}` in
  `env-vars` values (substituted the same controlled way `$HOME`/`${HOME}` is — no shell, no
  `eval`), and as an ordinary exported shell variable inside `shell-functions` bodies and
  `shell-cmd`:

  ```yaml
  shared:
    shell-functions:
      cd_project: |
        cd "$_PROJECT"

  envs:
    dev:
      project: $HOME/projects/dev
      env-vars:
        KUBECONFIG: $HOME/.kube/config
        CODEX_HOME: $_PROJECT/.codex
        CODEX_SQLITE_HOME: $_PROJECT/.codex/sqlite
  ```

- **`__ENV_SWITCHER_ACTIVE_PROJECT`** is exported after your own functions/variables and before
  `shell-cmd` runs, so a hook can tell which project just activated:

  ```yaml
  shared:
    shell-cmd: |
      printf 'env-switcher: activated %s\n' "$__ENV_SWITCHER_ACTIVE_PROJECT"
  ```

- **`__ENV_SWITCHER_TARGET_SHELL`** never reaches the interactive shell at all — it's a
  single-command-scoped assignment (`__ENV_SWITCHER_TARGET_SHELL=bash
  "$HOME/.env-switcher/bin/env-switcher" "$@"`, not `export`) that exists only for the duration of
  that one binary invocation, telling the CLI which shell's rendering rules to use for that run. It
  shows up if you read the managed block (see [above](#how-switching-works)), but there's nothing
  to configure or read from it — it's not a variable your `env-vars`/`shell-functions` interact
  with.

Both `_PROJECT` and the `__ENV_SWITCHER_` prefix are checked at `validate` time, not only at
switch time — see
[settings-schema.md](specs/001-env-switching/contracts/settings-schema.md).

## Configuration examples

Shell functions are a single body offered to whichever shell is active (no separate `bash`/`zsh`
form); a function needing genuinely different behavior per shell branches on `$ZSH_VERSION` at
runtime. `shell-cmd` is an anonymous hook (shared and/or per-project, both run if both are set)
that executes as the last step of every switch — for one-off setup that doesn't need to be a
named, callable function. `_PROJECT` and `__ENV_SWITCHER_ACTIVE_PROJECT` are both usable here too
— see [Bundled environment variables](#bundled-environment-variables) above.

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
env-switcher upgrade          # or: env-switcher --upgrade
env-switcher upgrade --yes    # skip the confirmation, e.g. for scripted use
```

First checks and reports, on stderr, before touching anything:

```text
→ checking https://github.com/oleksdovz/env-switcher for the latest release
  current version   v1.0.0
  latest release    v1.2.0
  platform          darwin/arm64

Upgrade v1.0.0 -> v1.2.0? [y/N]
```

The version comparison is **semantic**, against the latest **stable** (non-draft, non-prerelease)
release of [oleksdovz/env-switcher](https://github.com/oleksdovz/env-switcher) — not just "most
recently published" — and reports `✓ already up to date (<version>)` and stops right there if
nothing newer is available. A locally built binary (an unreleased "dev" version, which isn't a
release to compare against) is always treated as behind, so it can still upgrade. Declining the
prompt (or an unrecognized argument) exits without changing anything.

Confirmed, it narrates each step on stderr as it happens (`→ downloading ...`, `→ verifying
checksum`, `→ extracting env-switcher`, `→ installing ...`), then:

1. picks the release asset matching the running `GOOS`/`GOARCH` (failing with an actionable
   message, listing what the release *does* publish, if there's no build for your platform);
2. downloads it to a temporary file in `~/.env-switcher/bin/`;
3. verifies it against the release's published `SHA256SUMS` — a release with no checksum file is
   treated as a hard failure, never installed unverified;
4. if the asset is an archive, extracts only the single expected `env-switcher` executable,
   rejecting absolute paths, `..` traversal, symlinks, or any other archive entry;
5. atomically replaces `~/.env-switcher/bin/env-switcher`, then prints (on stdout) the old
   version, new version, and installed path.

The existing binary is left untouched if any of these steps fails. `F6` in the interactive picker
(with its own confirmation dialog) drives the identical underlying check-then-install call — see
`internal/upgrade` and `internal/app/upgrade.go` — it's never reimplemented in the TUI; only the
CLI narrates it step by step, since the TUI has its own status line.

> `settings.yaml` and `current-env` are plaintext and not a secure secret store. Use short-lived
> credentials and a dedicated secret manager for sensitive environments.

See [configuration](docs/configuration.md), [operations](docs/operations.md), and
[security](docs/security.md).
