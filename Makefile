# Convenience wrapper around the scripts in ./scripts and the Go toolchain.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO      ?= go

.PHONY: all build test test-race vet fmt fmt-check lint clean release

all: fmt-check vet test build

build:
	$(GO) build -trimpath -ldflags "-s -w" -o bin/esurfingdialer ./cmd/esurfingdialer

test:
	$(GO) test ./...

test-race:
	$(GO) test -race -count=2 ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -s -w ./cmd ./internal

fmt-check:
	@out=$$(gofmt -s -l ./cmd ./internal); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Cross-compile everything the release workflow publishes.
release:
	VERSION=$(VERSION) ./scripts/build-release.sh

clean:
	rm -rf bin dist
