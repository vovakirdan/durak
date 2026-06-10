GO ?= go
BINARY ?= durak
GOCACHE ?= $(CURDIR)/.cache/go-build
GOMODCACHE ?= $(CURDIR)/.cache/go-mod

.PHONY: fmt
fmt:
	$(GO)fmt -w ./cmd ./internal

.PHONY: vet
vet:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) vet ./...

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
check: fmt vet test
