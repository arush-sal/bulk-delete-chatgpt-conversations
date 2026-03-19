BINARY    := chatgpt-bulk
MODULE    := github.com/arush-sal/bulk-delete-chatgpt-conversations
CMD       := ./cmd/chatgpt-bulk

OS         := $(shell uname -s | tr A-Z a-z)
ARCH       := $(shell uname -m | sed 's/x86_64/amd64/')
VERSION    := $(shell git describe --tags --always 2>/dev/null || echo dev)
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
ARCHS      := amd64 arm64

LDFLAGS := -X $(MODULE)/internal/version.Version=$(VERSION) \
           -X $(MODULE)/internal/version.GitCommit=$(GIT_COMMIT) \
           -X $(MODULE)/internal/version.BuildDate=$(BUILD_DATE)

.PHONY: release build install clean fmt vet test snapshot release-dry-run build-all release-all build-amd64 build-arm64 release-amd64 release-arm64

release:
	if [ ! -d dist ]; then mkdir dist; fi
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	tar -czf dist/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz \
		README.md LICENSE $(BINARY)
	shasum -a 256 dist/$(BINARY)_$(VERSION)_$(OS)_$(ARCH).tar.gz > dist/checksums.txt

build-amd64:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_amd64 $(CMD)

build-arm64:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY)_arm64 $(CMD)

build-all: build-amd64 build-arm64

release-amd64:
	if [ ! -d dist ]; then mkdir dist; fi
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	tar -czf dist/$(BINARY)_$(VERSION)_$(OS)_amd64.tar.gz \
		README.md LICENSE $(BINARY)
	shasum -a 256 dist/$(BINARY)_$(VERSION)_$(OS)_amd64.tar.gz >> dist/checksums.txt

release-arm64:
	if [ ! -d dist ]; then mkdir dist; fi
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)
	tar -czf dist/$(BINARY)_$(VERSION)_$(OS)_arm64.tar.gz \
		README.md LICENSE $(BINARY)
	shasum -a 256 dist/$(BINARY)_$(VERSION)_$(OS)_arm64.tar.gz >> dist/checksums.txt

release-all:
	if [ ! -d dist ]; then mkdir dist; fi
	rm -f dist/checksums.txt
	$(MAKE) release-amd64
	$(MAKE) release-arm64

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
