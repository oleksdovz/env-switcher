# Configuration

The authoritative file is `~/.env-switcher/settings.yaml`, stored with mode `0600` under a `0700`
directory. Schema version 1 supports shared and per-project variables and shell functions; project
definitions override shared definitions with the same name.

Use the starter file as the canonical example. Values are literal: env-switcher performs no shell,
command, or glob expansion in values, and no arbitrary variable expansion — with two narrow,
specific exceptions, both handled in Go, never by invoking a shell or `eval`:

- `project` supports a leading `~`/`~/` and any `$HOME`/`${HOME}` reference, expanded to the
  current user's home directory. It's still shown as configured (unexpanded) by `list`/`get`, but
  for switching it must resolve to a clean **absolute** path — an empty, relative, or otherwise
  unresolved `project` fails the switch. Its resolved value is what populates `_PROJECT` (below);
  switching still never `cd`s to it, and it still doesn't need to exist.
- Every other `env-vars` value (shared and project-specific) may reference `$_PROJECT` or
  `${_PROJECT}`, which is substituted with that resolved absolute path. No other variable is
  expanded this way, and this substitution never touches `$(...)`, backticks, or any other shell
  syntax — a value containing those stays exactly as literal as it always has.

### `_PROJECT`

`_PROJECT` is a reserved, application-managed variable: env-switcher sets it automatically to the
environment's resolved `project` directory on every switch. Do not declare it yourself under
`env-vars` — settings written before this existed (or that still declare it as a manual
workaround) keep loading, but any declared value is always overwritten by the computed one, never
used. It's available to:

- other `env-vars` values, via `$_PROJECT`/`${_PROJECT}` substitution (see above);
- shared and project-specific `shell-functions`, as an ordinary exported shell variable — quote it
  when using it in a command, e.g. `cd "$_PROJECT"`;
- shared and project-specific `shell-cmd`, the same way.

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
