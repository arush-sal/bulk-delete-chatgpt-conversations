BINARY    := chatgpt-bulk
MODULE    := github.com/arush-sal/bulk-delete-chatgpt-conversations
CMD       := ./cmd/chatgpt-bulk

OS         := $(shell uname -s | tr A-Z a-z)
ARCH       := $(shell uname -m)
VERSION    := $(shell git describe --tags --always 2>/dev/null || echo dev)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
           -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: release build install clean fmt vet test snapshot release-dry-run

release:
	if [ ! -d dist ]; then mkdir dist; fi
	CGO_ENABLED=0 GOOS=$(OS) $GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	tar -czf dist/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz \
		README.md LICENSE $(BINARY)
	sha256sum dist/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz > dist/checksum.txt

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
