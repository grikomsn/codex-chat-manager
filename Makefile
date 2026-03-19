GO ?= go
BINARY ?= codex-chat-manager
CMD := ./cmd/$(BINARY)
BIN_DIR := ./bin
OUT := $(BIN_DIR)/$(BINARY)
CODEX_HOME ?= $(HOME)/.codex
ARGS ?=

.PHONY: help build install run tui list archive unarchive delete resume fmt tidy test test-race vet verify check clean

help:
	@printf '%s\n' \
		'Targets:' \
		'  make build       Build ./bin/$(BINARY)' \
		'  make install     Install the binary with go install' \
		'  make run         Run the TUI (same as tui)' \
		'  make tui         Run the TUI against CODEX_HOME' \
		'  make list        Run sessions list with optional ARGS' \
		'  make archive     Run sessions archive with ARGS="--id ..."' \
		'  make unarchive   Run sessions unarchive with ARGS="--id ..."' \
		'  make delete      Run sessions delete with ARGS="--id ... --yes"' \
		'  make resume      Run sessions resume with ARGS="--id ..."' \
		'  make fmt         Run gofmt on cmd/ and internal/' \
		'  make tidy        Run go mod tidy' \
		'  make test        Run go test ./...' \
		'  make test-race   Run go test -race ./...' \
		'  make vet         Run go vet ./...' \
		'  make verify      Run test, vet, and test-race without mutating files' \
		'  make check       Run fmt, tidy, verify' \
		'  make clean       Remove build artifacts'

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(OUT) $(CMD)

install:
	$(GO) install $(CMD)

run: tui

tui:
	$(GO) run $(CMD) tui --codex-home "$(CODEX_HOME)" $(ARGS)

list:
	$(GO) run $(CMD) sessions list --codex-home "$(CODEX_HOME)" $(ARGS)

archive:
	$(GO) run $(CMD) sessions archive --codex-home "$(CODEX_HOME)" $(ARGS)

unarchive:
	$(GO) run $(CMD) sessions unarchive --codex-home "$(CODEX_HOME)" $(ARGS)

delete:
	$(GO) run $(CMD) sessions delete --codex-home "$(CODEX_HOME)" $(ARGS)

resume:
	$(GO) run $(CMD) sessions resume --codex-home "$(CODEX_HOME)" $(ARGS)

fmt:
	gofmt -w cmd internal

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

verify: test vet test-race

check: fmt tidy verify

clean:
	rm -rf $(BIN_DIR)
	rm -f ./$(BINARY)
