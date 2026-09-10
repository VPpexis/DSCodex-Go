BINARY  := dscodex
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)
GITLEAKS ?= gitleaks

ifeq ($(OS),Windows_NT)
HOST_BINARY := $(BINARY).exe
CROSS_BUILD = set GOOS=$(1)&& set GOARCH=$(2)&& go build -trimpath -o dist/$(BINARY)-$(1)-$(2)$(3) ./cmd/dscodex
else
HOST_BINARY := $(BINARY)
CROSS_BUILD = GOOS=$(1) GOARCH=$(2) go build -trimpath -o dist/$(BINARY)-$(1)-$(2)$(3) ./cmd/dscodex
endif

.PHONY: all build test fmt vet lint tidy clean cross hooks secrets

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(HOST_BINARY) ./cmd/dscodex

test:
	go test -race ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist

cross:
	$(call CROSS_BUILD,darwin,amd64,)
	$(call CROSS_BUILD,darwin,arm64,)
	$(call CROSS_BUILD,linux,amd64,)
	$(call CROSS_BUILD,linux,arm64,)
	$(call CROSS_BUILD,windows,amd64,.exe)
	$(call CROSS_BUILD,windows,arm64,.exe)

hooks:
	git config core.hooksPath .githooks
	@echo pre-commit hook installed: .githooks/pre-commit (gitleaks, or pattern fallback)

secrets:
	$(GITLEAKS) git --redact --verbose
