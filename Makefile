.PHONY: build build-all test lint lint-fix clean install fmt tidy vet check help

BINARY := jiratui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"
GOLANGCI_LINT := golangci-lint

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-all: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .

test:
	go test -race ./...

lint:
	$(GOLANGCI_LINT) run

lint-fix:
	$(GOLANGCI_LINT) run --fix

clean:
	rm -f $(BINARY)
	rm -rf dist/

install:
	go install $(LDFLAGS) .

fmt:
	go fmt ./...

tidy:
	go mod tidy

vet:
	go vet ./...

check: fmt tidy vet lint test

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build for current platform"
	@echo "  build-all   Cross-compile for all platforms"
	@echo "  test        Run tests"
	@echo "  lint        Run linter"
	@echo "  lint-fix    Run linter with auto-fix"
	@echo "  clean       Clean build artefacts"
	@echo "  install     Install to $$GOPATH/bin"
	@echo "  fmt         Format Go code"
	@echo "  tidy        Tidy Go modules"
	@echo "  vet         Run go vet"
	@echo "  check       Run all checks (fmt, tidy, vet, lint, test)"
	@echo "  help        Show this help message"
