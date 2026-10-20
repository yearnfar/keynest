SHELL := /bin/sh

WAILS ?= wails
GO ?= go
DEV_FLAGS ?=
BUILD_FLAGS ?=
UPDATER_PACKAGE := github.com/yearnfar/keynest/internal/updater
DEV_VERSION := $(shell git rev-parse --short=8 HEAD 2>/dev/null || printf unknown)
DEV_LDFLAGS := -X $(UPDATER_PACKAGE).Version=$(DEV_VERSION) -X $(UPDATER_PACKAGE).BuildMode=development

.DEFAULT_GOAL := help

.PHONY: help check-wails dev debug build-debug release test

help:
	@echo "KeyNest build commands:"
	@echo "  make dev          Start Wails development mode with hot reload"
	@echo "  make debug        Start development mode with debug logging"
	@echo "  make build-debug  Build a package with debug tools enabled"
	@echo "  make release      Build a clean release package"
	@echo "  make test         Run all Go tests"
	@echo ""
	@echo "Optional: DEV_FLAGS='...' or BUILD_FLAGS='...'"

check-wails:
	@command -v $(WAILS) >/dev/null 2>&1 || { echo "Wails CLI is required: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }

dev: check-wails
	VITE_APP_VERSION=$(DEV_VERSION) $(WAILS) dev -ldflags "$(DEV_LDFLAGS)" $(DEV_FLAGS)

debug: check-wails
	VITE_APP_VERSION=$(DEV_VERSION) $(WAILS) dev -loglevel Debug -ldflags "$(DEV_LDFLAGS)" $(DEV_FLAGS)

build-debug: check-wails
	$(WAILS) build -clean -debug $(BUILD_FLAGS)

release: check-wails
	WAILS=$(WAILS) node scripts/build-release.mjs -clean -trimpath $(BUILD_FLAGS)

test:
	$(GO) test ./...
