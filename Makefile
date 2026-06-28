.PHONY: build test clean install build-tui

BINARY=skillvault
INSTALL_DIR=$(HOME)/tools

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
