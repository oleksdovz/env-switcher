# Security model

- Configuration is local plaintext, not a secret manager.
- F2 and F3 are the only intentional full-file disclosure paths.
- Diagnostics, errors, logs, crash reports, process arguments, profile blocks, and user-visible
  shell-integration output must not contain configured values.
- A successful switch writes quoted values to `~/.env-switcher/current-env` (`0600`) because the
  wrapper must apply them; the file is overwritten atomically and only on success. Only a bare
  invocation, a project name, or `--select` ever write it — every other command leaves it
  untouched — and the wrapper clears it *before* each invocation and sources it afterward only if
  that invocation just rewrote it, so a failed switch or a non-switch command never re-applies a
  stale prior switch. `current-env` is as sensitive as `settings.yaml` and should be treated the
  same way.
- Upgrading never installs an unverified binary: a release with no published checksum file, or a
  downloaded asset whose checksum doesn't match, is a hard failure, not a skipped check. Downloads
  and API requests use HTTPS with redirects restricted to GitHub's own hosts, and the previously
  installed executable is left untouched if any step of an upgrade fails.
- Function bodies (and the anonymous `shell-cmd` hook) are trusted executable user code and run
  only after explicit environment selection, with no separate confirmation step — configuring one
  is itself the trust decision, the same way configuring an `env-vars` value is. Syntax validation
  is not a security sandbox.
- `__ENV_SWITCHER_ACTIVE_PROJECT` uses a reserved prefix. Switching applies only the newly selected
  project's own variables/functions/shell-cmd on top of the shell's existing state — it does not
  track or remove anything a previously active project set, and it is not an all-or-nothing
  transaction: a single `readonly`-name conflict fails only that one statement, not the whole
  switch.

Prefer workload identity, short-lived credentials, OS keychains, or a dedicated secret manager over
long-lived plaintext access keys. Review profile backups as sensitive files.
