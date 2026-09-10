BINARY  := dscodex
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build test fmt vet tidy clean cross hooks secrets

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/dscodex

test:
	go test -race ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin dist

cross:
	GOOS=darwin  GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-darwin-amd64 ./cmd/dscodex
	GOOS=darwin  GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-darwin-arm64 ./cmd/dscodex
	GOOS=linux   GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-linux-amd64 ./cmd/dscodex
	GOOS=linux   GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-linux-arm64 ./cmd/dscodex
	GOOS=windows GOARCH=amd64 go build -trimpath -o dist/$(BINARY)-windows-amd64.exe ./cmd/dscodex
	GOOS=windows GOARCH=arm64 go build -trimpath -o dist/$(BINARY)-windows-arm64.exe ./cmd/dscodex

hooks:
	git config core.hooksPath .githooks
	@echo "pre-commit hook installed: .githooks/pre-commit (gitleaks, or pattern fallback)"

secrets:
	@command -v gitleaks >/dev/null 2>&1 || (echo "gitleaks not installed: https://github.com/gitleaks/gitleaks"; exit 1)
	gitleaks detect --source . --redact --verbose
