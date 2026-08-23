# Contract: `settings.yaml`

## Location and Permissions

- Default: `~/.env-switcher/settings.yaml`.
- Directory `0700`, file `0600` on POSIX systems.
- Owner must be current user; group/other write permissions block function activation.
- One UTF-8 YAML document, maximum 1 MiB.

## Version 1 Shape

```yaml
version: 1

shared:
  env-vars: &shared_env_vars
    SHARED_EXAMPLE: placeholder-only
    AWS_REGION: eu-west-1
  shell-functions: &shared_shell_functions
    k_load: |
      if [ -n "$ZSH_VERSION" ]; then
        source <(kubectl completion zsh)
        compdef _kubectl k
      else
        source <(kubectl completion bash)
        complete -o default -F __start_kubectl k
      fi
  shell-cmd: |
    export ENV_SWITCHER_SESSION="active"

envs:
  dev:
    project: ~/projects/dev
    env-vars:
      <<: *shared_env_vars
      PROJECT_DIR: ~/projects/dev
      AWS_ACCESS_KEY_ID: EXAMPLE_ONLY
      AWS_SECRET_ACCESS_KEY: EXAMPLE_ONLY
    shell-functions:
      <<: *shared_shell_functions
      k_ns: |
        kubectl get namespaces
    shell-cmd: |
      export DEV_TOOLS_ENABLED="true"

  staging:
    project: ~/projects/staging
    env-vars:
      <<: *shared_env_vars
      PROJECT_DIR: ~/projects/staging
      AWS_ACCESS_KEY_ID: EXAMPLE_ONLY
      AWS_SECRET_ACCESS_KEY: EXAMPLE_ONLY

  prod:
    project: ~/projects/prod
    env-vars:
      <<: *shared_env_vars
      PROJECT_DIR: ~/projects/prod
      AWS_ACCESS_KEY_ID: EXAMPLE_ONLY
      AWS_SECRET_ACCESS_KEY: EXAMPLE_ONLY
```

All credentials are deliberately non-functional. Plaintext YAML is not a secure secret store.
`dev`'s effective `AWS_REGION` is the shared `eu-west-1` (not overridden here); its `shell-functions`
include both the merged `k_load` and its own `k_ns`; its `shell-cmd` runs after the shared one.

## Rules

- Root keys: `version`, optional `shared`, and `envs` only.
- Shared/project keys: `env-vars`, `shell-functions`, `shell-cmd`, and project-level `project` only.
- `project` is required, non-empty, informational metadata (shown by `list`/`get`) — switching
  never `cd`s to it or checks that it exists.
- Variables/functions use maps; names are case-sensitive.
- Reject unknown or duplicate keys, multiple documents, custom tags, and type mismatches.
- Anchors (`&name`), aliases (`*name`), and merge keys (`<<: *name`) are accepted — explicit keys in
  a mapping win over merged ones, standard YAML merge-key semantics. Two safety rules bound them:
  an anchored value cannot itself reference another anchor (one hop only, never chained), and the
  document's total size after every alias is resolved must not exceed 8 MiB (independent of the
  1 MiB raw-file cap).
- Names match `[A-Za-z_][A-Za-z0-9_]*` and cannot use `__ENV_SWITCHER_`.
- Project names additionally cannot be a reserved CLI command word (`help`, `list`, `ls`, `edit`,
  `get`, `version`, `validate`, `install`, `rollback`, `uninstall`, `reload`, `view`), since a
  project sharing one would be unreachable through `env-switcher <project>`.
- A name cannot be both variable and function in one effective project.
- Values are literal strings: no interpolation, command substitution, or implicit conversion.
- A `shell-functions` entry and `shell-cmd` are each a single scalar body — there is no per-shell
  (`bash`/`zsh`) split. The same body is offered to, and syntax-checked against, whichever shell is
  active; a function that must genuinely differ per shell branches on something like
  `$ZSH_VERSION` at runtime, as `k_load` does above.
- `shell-cmd` is anonymous (no name) and additive: if both `shared` and a project define one, the
  shared body runs first, then the project's own — neither replaces the other.
- Function/shell-cmd bodies are trusted code, syntax-checked for the active shell without
  execution.
- Project definitions override same-kind shared `env-vars`/`shell-functions` by exact name.

The product warns on first run and whenever a deterministic digest of configured shell functions
changes. Viewing, editing, validating, reloading, or hashing the file never executes a function;
function definitions are applied only after explicit environment selection. Acknowledgement persists
as user-only metadata containing a schema version and digest, never bodies or secret values.

Success yields one immutable Settings model. Failure yields source-located, redacted errors on
stderr and produces no partial model or payload. Versions other than `1` fail with compatibility
guidance; automatic migration is outside v1.
