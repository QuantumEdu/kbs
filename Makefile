.PHONY: build test clean install install-all \
        build-tui build-q-secrets install-q-secrets \
        build-telemetry install-telemetry \
        test-integration

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

test-integration:
	go test -tags integration -count=1 ./internal/agenttelemetry/...

clean:
	rm -f $(BINARY) $(BINARY)-tui telemetryd telemetryctl telemetrywrap
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

# Telemetry binaries
build-telemetry:
	go build -ldflags="-s -w" -o telemetryd ./cmd/telemetryd
	go build -ldflags="-s -w" -o telemetryctl ./cmd/telemetryctl
	go build -ldflags="-s -w" -o telemetrywrap ./internal/agenttelemetry/telemetrywrap
	@echo "Built telemetry binaries"

install-telemetry: build-telemetry
	mkdir -p $(INSTALL_DIR)
	cp telemetryd $(INSTALL_DIR)/telemetryd
	cp telemetryctl $(INSTALL_DIR)/telemetryctl
	cp telemetrywrap $(INSTALL_DIR)/telemetrywrap
	@echo "Installed telemetry binaries to $(INSTALL_DIR)/"
	@echo ""
	@echo "Alternative: run 'skillvault install-telemetry' or 'skillvault init --with-telemetry' to install from within kbs."

install-all: install install-q-secrets install-telemetry
	@echo "All binaries installed to $(INSTALL_DIR)/"
