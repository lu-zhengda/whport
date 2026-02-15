.PHONY: build run test lint clean install uninstall

BINARY     := whport
BUILD_DIR  := ./bin
PREFIX     ?= /usr/local
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w \
	-X github.com/lu-zhengda/whport/internal/cli.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/whport

run: build
	$(BUILD_DIR)/$(BINARY)

test:
	go test -race -cover ./...

lint:
	golangci-lint run ./...

install: build
	install -d $(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)
