# Envorca build and test targets.
#
# Default version is derived from git (tagged releases override both
# binaries); use `make build VERSION=x.y.z` to pin a specific version.

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo 0.1.0-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DAEMON_LDFLAGS = -s -w -X envorca.dev/envorca/daemon/internal/version.Version=$(VERSION) -X envorca.dev/envorca/daemon/internal/version.Commit=$(COMMIT)
CLI_LDFLAGS    = -s -w -X envorca.dev/envorca/cli/cmd/envorca.cliVersion=$(VERSION) -X envorca.dev/envorca/cli/cmd/envorca.cliCommit=$(COMMIT)
GO ?= go

.PHONY: all build test vet proto cross clean install

all: build

# Resolve workspace module versions, then wire everything up.
build:
	$(GO) work sync
	mkdir -p bin
	$(GO) -C daemon build -ldflags "$(DAEMON_LDFLAGS)" -o ../bin/envorcad ./cmd/envorcad
	$(GO) -C cli build -ldflags "$(CLI_LDFLAGS)" -o ../bin/envorca ./cmd/envorca

# Windows/amd64 binaries for the real target platform.
cross:
	$(GO) work sync
	mkdir -p bin
	cd daemon && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(DAEMON_LDFLAGS)" -o ../bin/envorcad.exe ./cmd/envorcad
	cd cli && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(CLI_LDFLAGS)" -o ../bin/envorca.exe ./cmd/envorca

test:
	$(GO) work sync
	$(GO) -C api test ./...
	$(GO) -C daemon test ./...
	$(GO) -C cli test ./...

vet:
	$(GO) work sync
	$(GO) -C api vet ./...
	$(GO) -C daemon vet ./...
	$(GO) -C cli vet ./...

# Regenerate gRPC code from api/proto (requires protoc + plugins).
proto:
	$(GO) work sync
	./scripts/gen-proto.sh

# Install both binaries into $HOME/go/bin (or $(PREFIX)/bin).
PREFIX ?= $(HOME)/go
install: build
	mkdir -p $(PREFIX)/bin
	cp bin/envorcad bin/envorca $(PREFIX)/bin/

clean:
	rm -rf bin