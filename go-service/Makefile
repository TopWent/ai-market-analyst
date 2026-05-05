.PHONY: build test test-race vet lint fmt cover clean

GO         ?= go
GOLANGCI   ?= golangci-lint
BIN        ?= market-data
PKG        ?= ./cmd/market-data
VERSION    ?= dev
LDFLAGS    := -s -w -X main.version=$(VERSION)

build:
	$(GO) build -trimpath -ldflags='$(LDFLAGS)' -o $(BIN) $(PKG)

test:
	$(GO) test -count=1 ./...

test-race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

lint:
	$(GOLANGCI) run ./...

fmt:
	$(GO) fmt ./...

cover:
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BIN) coverage.out coverage.html
