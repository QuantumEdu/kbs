.PHONY: build test clean install build-tui build-q-secrets install-q-secrets install-all

BINARY=skillvault
INSTALL_DIR=$(HOME)/tools
Q_SECRETS_DIR=q-secrets
Q_SECRETS_BIN=q-secrets

build:
	go build -o $(BINARY) ./cmd/skillvault

# build-tui compiles the binary WITH the Bubble Tea TUI enabled.
# The TUI and its dependencies (bubbletea, lipgloss, bubbles) are
# gated behind the "tui" build tag — zero cost in the default binary.
build-tui:
	go build -tags tui -o $(BINARY)-tui ./cmd/skillvault

test:
	go test ./...

test-cover:
	go test -cover ./...

clean:
	rm -f $(BINARY) $(BINARY)-tui
	go clean -cache

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"

build-q-secrets:
	cd $(Q_SECRETS_DIR) && go build -o $(Q_SECRETS_BIN) .

install-q-secrets: build-q-secrets
	mkdir -p $(INSTALL_DIR)
	cp $(Q_SECRETS_DIR)/$(Q_SECRETS_BIN) $(INSTALL_DIR)/$(Q_SECRETS_BIN)
	@echo "Installed q-secrets to $(INSTALL_DIR)/$(Q_SECRETS_BIN)"
	@echo ""
	@echo "Alternative: run 'skillvault init --with-secrets' or 'skillvault secrets install' to install from within kbs."

install-all: install install-q-secrets

test-integration:
	go test -tags integration -count=1 ./internal/agenttelemetry/...
