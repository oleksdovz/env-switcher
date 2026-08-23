# US1 validation

- [x] Bash and Zsh current-shell activation tests pass on macOS arm64.
- [x] Exact 100-switch tests pass for Bash and Zsh with obsolete cleanup and unrelated-state preservation.
- [x] Readonly-variable forced failure leaves directory and earlier modified values unchanged.
- [x] Payload envelope, quoting, NUL rejection, malformed wrapper payload rejection, and syntax-only function validation are tested.
- [x] Equivalent native Linux Bash/Zsh container evidence passes on Linux arm64; CI workflow covers hosted runners.
