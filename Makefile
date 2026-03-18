BINARY    := chatgpt-bulk
MODULE    := github.com/arush-sal/bulk-delete-chatgpt-conversations
CMD       := ./cmd/chatgpt-bulk

VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
           -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: build install clean fmt vet test snapshot release-dry-run

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

install:
	go install -ldflags "$(LDFLAGS)" $(CMD)

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test ./...

snapshot:
	goreleaser release --snapshot --clean

release-dry-run:
	goreleaser release --skip=publish --clean
