## Summary

<!-- Describe behavior and compatibility impact. -->

## Constitution impact

- [ ] I. Portable Go CLI
- [ ] II. Safe Shell Integration
- [ ] III. Configuration Is the Source of Truth
- [ ] IV. Secrets Stay Local and Protected
- [ ] V. Tested and Predictable Terminal UX

Explain each affected principle and any exception. Exceptions require rationale and a removal plan.

## Validation evidence

- [ ] `gofmt -l .` is empty
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] Relevant Bash/Zsh integration tests
- [ ] Relevant Linux/macOS CI jobs
- [ ] Quoting, permissions, injection, idempotency, backup, and rollback review when applicable

Do not paste settings, activation payloads, function bodies, or secret-bearing diagnostics here.
