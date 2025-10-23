# GoCarPlay Makefile
BIN := gocarplay
SERVER_BIN := gocarplay-server
GIT_REV := $(shell git describe --tags --always 2>/dev/null)
ifdef GIT_REV
LDFLAGS := -X main.version=$(GIT_REV)
else
LDFLAGS :=
endif
BUILDFLAGS := -tags netgo
# Static build flags - mostly static binaries
# Note: libusb-1.0 requires libudev when building statically on Linux
# We omit -ludev and let the linker find it automatically
STATIC_LDFLAGS := $(LDFLAGS) -linkmode external -extldflags \"-static\"
STATIC_BUILDFLAGS := $(BUILDFLAGS) -tags "netgo osusergo"
SERVER_DIR := ./cmd/server
LIB_PACKAGES := ./... ./protocol/... ./link/...

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt

.PHONY: build build-static server host arm amd64 arm64 dist clean test fmt tidy vet lint all run run-server install deps static static-host static-amd64 static-arm static-arm-native static-arm64 static-darwin-amd64 static-darwin-arm64 static-windows static-all

# Default target
all: tidy fmt vet test build

# Build library only (no binary, just check compilation)
build:
	$(GOBUILD) -v ./...

# Static build for library (compile check with static flags)
build-static:
	CGO_ENABLED=1 $(GOBUILD) $(STATIC_BUILDFLAGS) -v ./...

# Build server application for host platform
server: host

host:
	cd $(SERVER_DIR) && $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-host .

# Cross-compilation targets
# Note: CGO_ENABLED=1 is required for USB access via gousb
# For cross-compilation, you need the appropriate toolchain:
#   - ARM: arm-linux-gnueabihf-gcc
#   - ARM64: aarch64-linux-gnu-gcc
# If you don't have the toolchain, build on the target device instead
amd64:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-amd64 .

arm:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-arm .

arm64:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-arm64 .

# Simple ARM build (build on the target device)
arm-native:
	cd $(SERVER_DIR) && $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-arm .

# macOS targets
darwin-amd64:
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-darwin-amd64 .

darwin-arm64:
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-darwin-arm64 .

# Windows targets
windows:
	cd $(SERVER_DIR) && GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ../../$(SERVER_BIN)-windows.exe .

# ============================================================================
# Static Build Targets
# ============================================================================
# Static builds create binaries with all dependencies statically linked
# Note: Requires static C libraries (libusb, etc.) for CGO dependencies
# On Linux: apt-get install libusb-1.0-0-dev (or equivalent)
# On macOS: brew install libusb

# Static build for host platform
static-host:
	cd $(SERVER_DIR) && CGO_ENABLED=1 $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-host .

# Static build for Linux AMD64
static-amd64:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-amd64 .

# Static build for Linux ARM
static-arm:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-arm .

# Static build for Linux ARM64
static-arm64:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-arm64 .

# Static build on ARM device (no cross-compilation)
static-arm-native:
	cd $(SERVER_DIR) && CGO_ENABLED=1 $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-arm .

# Static builds for macOS (Note: macOS doesn't support fully static binaries)
static-darwin-amd64:
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-darwin-amd64 .

static-darwin-arm64:
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(BUILDFLAGS) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-darwin-arm64 .

# Static build for Windows
static-windows:
	cd $(SERVER_DIR) && GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc $(GOBUILD) $(STATIC_BUILDFLAGS) -ldflags "$(STATIC_LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-static-windows.exe .

# Build all static binaries
static-all:
	@echo "Building all static binaries..."
	$(MAKE) static-amd64
	$(MAKE) static-arm
	$(MAKE) static-arm64
	$(MAKE) static-darwin-amd64
	$(MAKE) static-darwin-arm64
	@echo "✅ All static builds complete!"

# Quick static build alias
static: static-host

# Distribution build (optimized, stripped)
# CGO_ENABLED=1 is required for USB access
dist:
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-arm-dist .

dist-native:
	cd $(SERVER_DIR) && $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-dist .

dist-all: dist
	cd $(SERVER_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-amd64-dist .
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-arm64-dist .
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-darwin-amd64-dist .
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) -ldflags "$(LDFLAGS) -s -w" -o ../../$(SERVER_BIN)-darwin-arm64-dist .

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(SERVER_BIN)-*
	rm -f coverage.txt coverage.html

# Run tests
test:
	$(GOTEST) -v -race -coverprofile=coverage.txt ./...

# Run tests with coverage report
test-coverage: test
	$(GOCMD) tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Format code
fmt:
	$(GOFMT) -s -w .
	$(GOCMD) fmt ./...

# Run go mod tidy
tidy:
	$(GOMOD) tidy

# Run go vet
vet:
	$(GOCMD) vet ./...

# Install golangci-lint and run linting
lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# Update dependencies
deps:
	$(GOGET) -u ./...
	$(GOMOD) tidy

# Run the server application
run: host
	./$(SERVER_BIN)-host

# Run server with custom configuration
run-server: host
	@echo "Starting GoCarPlay MJPEG Server on http://localhost:8001"
	./$(SERVER_BIN)-host

# Run server with Android Auto mode enabled
run-android: host
	@echo "Starting with Android Auto mode enabled"
	ANDROID_MODE=true ./$(SERVER_BIN)-host

# Run server with iOS/CarPlay mode
run-ios: host
	@echo "Starting with iOS/CarPlay mode"
	ANDROID_MODE=false ./$(SERVER_BIN)-host

# Development helpers
dev: fmt vet test run-server

# Watch for changes and rebuild (requires entr)
watch:
	@which entr > /dev/null 2>&1 || (echo "Please install entr: brew install entr (macOS) or apt-get install entr (Linux)" && exit 1)
	find . -name "*.go" | entr -r make run-server

# Install binary to system (requires sudo)
install: dist
	sudo cp $(SERVER_BIN)-arm-dist /usr/local/bin/$(SERVER_BIN)
	sudo chmod +x /usr/local/bin/$(SERVER_BIN)

# Install for development (to GOPATH/bin)
install-dev: host
	mkdir -p $(GOPATH)/bin
	cp $(SERVER_BIN)-host $(GOPATH)/bin/$(SERVER_BIN)

# Docker targets
docker-build:
	docker build -t gocarplay:latest .

docker-run:
	docker run --rm -it --privileged -v /dev/bus/usb:/dev/bus/usb -p 8001:8001 gocarplay:latest

# Documentation
docs:
	@echo "Generating documentation..."
	godoc -http=:6060 &
	@echo "Documentation server started at http://localhost:6060/pkg/github.com/mzyy94/gocarplay/"

# Check code quality
check: fmt vet lint test
	@echo "✅ All checks passed!"

# Show help
help:
	@echo "GoCarPlay Makefile targets:"
	@echo ""
	@echo "Building:"
	@echo "  make build       - Build the library"
	@echo "  make build-static - Build library with static flags"
	@echo "  make server      - Build MJPEG server for host platform"
	@echo "  make host        - Build server for host platform"
	@echo "  make amd64       - Build for Linux AMD64"
	@echo "  make arm         - Build for Linux ARM (cross-compile)"
	@echo "  make arm-native  - Build for ARM on ARM device"
	@echo "  make arm64       - Build for Linux ARM64"
	@echo "  make darwin-*    - Build for macOS"
	@echo "  make windows     - Build for Windows"
	@echo "  make dist        - Build optimized ARM binary"
	@echo "  make dist-native - Build optimized for current platform"
	@echo "  make dist-all    - Build optimized for all platforms"
	@echo ""
	@echo "Static Builds:"
	@echo "  make static      - Build static binary for host platform"
	@echo "  make static-host - Build static binary for host platform"
	@echo "  make static-amd64 - Build static for Linux AMD64"
	@echo "  make static-arm  - Build static for Linux ARM"
	@echo "  make static-arm-native - Build static on ARM device"
	@echo "  make static-arm64 - Build static for Linux ARM64"
	@echo "  make static-darwin-* - Build for macOS (semi-static)"
	@echo "  make static-windows - Build static for Windows"
	@echo "  make static-all  - Build all static binaries"
	@echo ""
	@echo "Testing & Quality:"
	@echo "  make test        - Run tests"
	@echo "  make test-coverage - Run tests with coverage"
	@echo "  make bench       - Run benchmarks"
	@echo "  make fmt         - Format code"
	@echo "  make vet         - Run go vet"
	@echo "  make lint        - Run linter"
	@echo "  make check       - Run all quality checks"
	@echo ""
	@echo "Running:"
	@echo "  make run         - Run the MJPEG server"
	@echo "  make run-server  - Run MJPEG server on http://localhost:8001"
	@echo "  make run-android - Run with Android mode"
	@echo "  make run-ios     - Run with iOS mode"
	@echo "  make watch       - Auto-rebuild on changes"
	@echo ""
	@echo "Maintenance:"
	@echo "  make tidy        - Run go mod tidy"
	@echo "  make deps        - Update dependencies"
	@echo "  make clean       - Remove build artifacts"
	@echo ""
	@echo "Installation:"
	@echo "  make install     - Install to /usr/local/bin"
	@echo "  make install-dev - Install to GOPATH/bin"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build - Build Docker image"
	@echo "  make docker-run  - Run in Docker container"
	@echo ""
	@echo "Other:"
	@echo "  make docs        - Start documentation server"
	@echo "  make help        - Show this help"

.DEFAULT_GOAL := help