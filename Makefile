.PHONY: test watch lint fmt build install secrets check setup

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

test:            ## run all tests
	gotestsum --format testname -- -race ./...

watch:           ## re-run tests on every save
	gotestsum --watch --format testname -- ./...

lint:            ## vet + golangci-lint
	go vet ./...
	golangci-lint run

fmt:             ## format code
	golangci-lint fmt

build:           ## build ./bin/delegate
	go build -ldflags "-X main.version=$(VERSION)" -o bin/delegate ./cmd/delegate

install:         ## install delegate into $GOPATH/bin
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/delegate

secrets:         ## scan the whole git history for secrets
	gitleaks git --redact --verbose

check: test lint secrets  ## everything CI runs

setup:           ## one-time: install pre-commit hooks (lint + secret scan)
	pre-commit install
