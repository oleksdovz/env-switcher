# US3 validation

- [x] Repeated installation is byte-idempotent and produces exactly one managed block.
- [x] Profile bytes and mode are preserved; symlinks and malformed markers fail closed.
- [x] Backup digest/scope validation, rollback, uninstall, and lock contention are tested.
- [x] Installed Bash and Zsh wrappers were smoke-tested in isolated temporary homes on macOS.
- [x] Equivalent native Linux install lifecycle tests pass in a read-only-mounted Linux arm64 container.
