GO ?= go
LINTER ?= golangci-lint
BINARY ?= durak
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.cache/golangci-lint

.PHONY: fmt
fmt:
	$(GO)fmt -w ./cmd ./internal

.PHONY: vet
vet:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

.PHONY: lint
lint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) $(LINTER) run

.PHONY: test
test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

.PHONY: build
build:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) build -o bin/$(BINARY) ./cmd/durak

.PHONY: run
run:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) run ./cmd/durak

.PHONY: check
check: fmt lint test
	@echo "Do not forget to call sentrux.scan!"
