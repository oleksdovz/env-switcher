# Foundation validation

- [x] Strict valid/invalid YAML tests pass with duplicate, alias, merge, unknown-field, implicit-type, size, and permission rejection.
- [x] Starter configuration has two projects and placeholder-only credentials.
- [x] Function hashing and configuration validation execute no configured function bodies.
- [x] `go test ./internal/config ./internal/app` passes on macOS arm64 with Go 1.26.1.
- [x] `go vet ./...` passes locally.
