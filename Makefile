.PHONY: build test clean install

BINARY=skillvault
INSTALL_DIR=$(HOME)/tools

build:
	go build -o $(BINARY) ./cmd/skillvault

test:
	go test ./...

test-cover:
	go test -cover ./...

clean:
	rm -f $(BINARY)
	go clean -cache

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"
