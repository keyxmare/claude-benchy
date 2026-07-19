BINARY := benchy
IMAGE := claude-benchy:latest
IMAGE_GO := claude-benchy-go:latest

# Whole Go toolchain runs in Docker: nothing but docker is required on the host.
TOOLS := docker compose -f compose.tools.yaml
GO := $(TOOLS) run --rm go
LINT := $(TOOLS) run --rm lint

# Host platform for the `build` target so ./benchy runs natively without a Go
# toolchain installed. arm64 keeps its name; x86_64 maps to Go's amd64.
HOST_OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
HOST_ARCH := $(shell uname -m)
GOARCH := $(if $(filter x86_64,$(HOST_ARCH)),amd64,$(HOST_ARCH))

# Keychain service under which Claude Code stores its OAuth credentials on macOS.
KEYCHAIN_SERVICE := Claude Code-credentials
CLAUDE_DIR ?= $(HOME)/.claude

.PHONY: build serve serve-watch up down logs creds test fmt fmt-check vet lint check check-fast image image-go clean

build:
	$(GO) env GOOS=$(HOST_OS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $(BINARY) ./cmd/benchy

serve: build
	./$(BINARY) serve

# Dev dashboard: rebuild (Docker) and restart the native server on every change
# to a Go source or an embedded template/asset. Forward flags via ARGS, e.g.
# `make serve-watch ARGS="--addr 127.0.0.1:8080"`.
serve-watch:
	BINARY=./$(BINARY) ./scripts/serve-watch.sh $(ARGS)

up:
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f

# creds exports the Keychain-held credentials into ~/.claude so the dockerised
# dashboard can mount them. macOS keeps them in the Keychain, not on disk; this
# must run on the host (the Orbit task runner is a Linux container). A no-op
# where the file already exists (Linux, or an earlier export).
creds:
	@if [ -f "$(CLAUDE_DIR)/.credentials.json" ]; then \
		echo "creds already present: $(CLAUDE_DIR)/.credentials.json"; \
	elif [ "$(HOST_OS)" = "darwin" ]; then \
		mkdir -p "$(CLAUDE_DIR)"; \
		security find-generic-password -s "$(KEYCHAIN_SERVICE)" -w > "$(CLAUDE_DIR)/.credentials.json"; \
		chmod 600 "$(CLAUDE_DIR)/.credentials.json"; \
		echo "exported Keychain credentials to $(CLAUDE_DIR)/.credentials.json"; \
	else \
		echo "no credentials file and not on macOS: authenticate with claude on the host first"; \
		exit 1; \
	fi

test:
	$(GO) go test ./...

fmt:
	$(GO) gofmt -w .

fmt-check:
	@$(GO) sh -c 'out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi'

vet:
	$(GO) go vet ./...

lint:
	$(LINT) golangci-lint run

check: fmt-check vet lint test

check-fast: fmt-check vet lint

image:
	docker build -t $(IMAGE) build/docker

# Sandbox image bundling the Go toolchain, used by benches whose checks run
# go test/vet/build (e.g. benches/freedy, sandbox.image: claude-benchy-go:latest).
image-go:
	docker build -t $(IMAGE_GO) -f build/docker/Dockerfile.golang build/docker

clean:
	rm -f $(BINARY)
	rm -rf results
