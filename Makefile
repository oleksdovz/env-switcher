GO ?= go
GOFMT ?= gofmt

.PHONY: fmt vet test race build check

fmt:
	@test -z "$$($(GOFMT) -l .)"

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

race:
	$(GO) test -race ./...

build:
	$(GO) build -trimpath -o dist/env-switcher ./cmd/env-switcher

check: fmt vet test race build
