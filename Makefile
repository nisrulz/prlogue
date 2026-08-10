.PHONY: build install uninstall test test-live audit test-cover clean snapshot release help

BINARY=prlogue
DESTDIR=$(HOME)/go/bin
DESTDIR_SHOW=$(patsubst $(HOME)/%,~/%,$(DESTDIR))
.DEFAULT_GOAL := help

build:     ## Build the binary
	@echo "Building $(BINARY)..."
	@go build -o $(BINARY) .
	@echo "✓ Built $(BINARY)"

install: build  ## Install binary to ~/go/bin (no password needed)
	@echo "Installing $(BINARY) to $(DESTDIR_SHOW)..."
	@mkdir -p $(DESTDIR)
	@rm -f $(DESTDIR)/$(BINARY)
	@install -m 0755 $(BINARY) $(DESTDIR)/$(BINARY)
	@echo "✓ Installed $(BINARY) to $(DESTDIR_SHOW)/$(BINARY)"
	@case ":$$PATH:" in *":$(DESTDIR):"*) ;; *) \
		echo "  Tip: add $(DESTDIR_SHOW) to your PATH:"; \
		echo "  export PATH=\"$(DESTDIR_SHOW):\$$PATH\"" ;; esac

uninstall:  ## Remove binary from ~/go/bin
	@echo "Removing $(BINARY) from $(DESTDIR_SHOW)..."
	@rm -f $(DESTDIR)/$(BINARY)
	@echo "✓ Removed $(BINARY) from $(DESTDIR_SHOW)"

test:      ## Run tests (no cache)
	@echo "Running tests..."
	@go test -count=1 ./...

test-live:  ## End-to-end tests against real git repos (builds binary; uses a live provider or template fallback)
	@echo "Running end-to-end tests..."
	@scripts/live-test.sh

audit:     ## Run tests, vet, and the vulnerability scanner
	@echo "Running tests with the race detector..."
	@go test -race ./...
	@echo "Running go vet..."
	@go vet ./...
	@echo "Scanning for vulnerabilities..."
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		echo "  govulncheck not found; installing..."; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@govulncheck ./...
	@echo "✓ Audit passed"

test-cover:  ## Run tests with coverage
	@echo "Generating coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out | tail -1
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report written to coverage.html"

clean:     ## Remove build artifacts
	@echo "Cleaning up..."
	@go clean
	@rm -f $(BINARY) coverage.out coverage.html
	@echo "✓ Cleaned"

snapshot:  ## Test goreleaser build locally (no publish)
	@echo "Building a snapshot release (no publish)..."
	@goreleaser release --snapshot --clean

release:   ## Run full goreleaser release (requires tag)
	@echo "Running the release build..."
	@goreleaser release --clean

help:      ## Show this help
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*##' Makefile | sort | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'
