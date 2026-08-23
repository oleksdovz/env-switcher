# Configuration

The authoritative file is `~/.env-switcher/settings.yaml`, stored with mode `0600` under a `0700`
directory. Schema version 1 supports shared and per-project variables and shell functions; project
definitions override shared definitions with the same name.

Use the starter file as the canonical example. Values are literal: env-switcher performs no shell,
command, glob, variable, or tilde expansion in values. `project` is required but purely
informational (shown by `list`/`get`) — switching never changes the shell's directory or checks
that the path exists, so it may begin with `~/` or point anywhere at all.

YAML anchors (`&name`), aliases (`*name`), and merge keys (`<<: *name`) are supported — the starter
file uses them to share a common `env-vars`/`shell-functions` block across projects, with individual
projects overriding specific keys by naming them explicitly alongside the merge. Two safety limits
apply regardless: an anchored value cannot itself reference another anchor, and the document's
total resolved size (after expanding every alias) is capped independently of the raw file size — see
[settings-schema.md](../specs/001-env-switching/contracts/settings-schema.md) for the exact rules.

F2 intentionally displays the complete unmasked file only after a sensitive-data warning. F3 opens
the complete file in `VISUAL`, then `EDITOR`. These are explicit local disclosure actions.

Shell functions are trusted user-provided executable code. Each is a single body offered to
whichever shell is active — there's no separate `bash`/`zsh` form; a function that must behave
differently per shell branches on something like `$ZSH_VERSION` at runtime. `shell-cmd` is the same
kind of trusted code but anonymous: an unnamed hook that runs as the last step of every switch. If
both `shared` and a project define one, the shared hook runs first, then the project's own — neither
replaces the other. Bodies are syntax-checked but cannot be made semantically safe. A first-run or
changed-function warning (covering shell-functions and shell-cmd alike) must be acknowledged before
activation. Viewing, editing, validating, reloading, and digest calculation never execute them.

Project names cannot be `help`, `list`, `ls`, `edit`, `get`, `version`, `validate`, `install`,
`rollback`, `uninstall`, `reload`, or `view` — those words are reserved CLI commands, and a project
sharing one of them would be unreachable through `env-switcher <project>`. Use `env-switcher list`
to see configured projects and `env-switcher get <project>` to inspect one, including its resolved
(shared plus project-specific) variables and function bodies.
