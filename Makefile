BINARY := benchy
IMAGE := claude-benchy:latest

.PHONY: build test fmt fmt-check vet lint check image clean

build:
	go build -o $(BINARY) ./cmd/benchy

test:
	go test ./...

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping (vet still runs in check)"; \
	fi

check: fmt-check vet lint test

image:
	docker build -t $(IMAGE) build/docker

clean:
	rm -f $(BINARY)
	rm -rf results
