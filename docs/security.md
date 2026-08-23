# Security model

- Configuration is local plaintext, not a secret manager.
- F2 and F3 are the only intentional full-file disclosure paths.
- Diagnostics, errors, logs, crash reports, process arguments, profile blocks, acknowledgement
  metadata, and user-visible shell-integration output must not contain configured values.
- A successful switch writes quoted values to `~/.env-switcher/current-env` (`0600`) because the
  wrapper must apply them; the file is overwritten atomically and only on success, so a failed
  switch leaves the prior state in place and the wrapper's unconditional `source` re-applies it
  unchanged. `current-env` is as sensitive as `settings.yaml` and should be treated the same way.
- Function bodies (and the anonymous `shell-cmd` hook) are trusted executable user code and run
  only after explicit environment selection. Syntax validation is not a security sandbox.
- `__ENV_SWITCHER_ACTIVE_PROJECT` uses a reserved prefix. Switching applies only the newly selected
  project's own variables/functions/shell-cmd on top of the shell's existing state — it does not
  track or remove anything a previously active project set, and it is not an all-or-nothing
  transaction: a single `readonly`-name conflict fails only that one statement, not the whole
  switch.

Prefer workload identity, short-lived credentials, OS keychains, or a dedicated secret manager over
long-lived plaintext access keys. Review profile backups as sensitive files.
