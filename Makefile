.PHONY: build build-linux build-linux-docker build-darwin build-windows test lint clean install all deploy-linux version

BINARY_NAME=marmot
BUILD_DIR=bin

# Get version from git tag, fallback to "dev" if no tags exist
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
# Get short commit hash
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Get build time
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# LDFLAGS for version injection
LDFLAGS=-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

build: version
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) -ldflags "$(LDFLAGS)" ./cmd/marmot
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME) (version: $(VERSION))"

build-linux: version
	@echo "Building $(BINARY_NAME) for Linux..."
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 -ldflags "$(LDFLAGS)" ./cmd/marmot
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 (version: $(VERSION))"

build-linux-docker: version
	@echo "Building $(BINARY_NAME) for Linux AMD64 using Docker..."
	@docker run --rm --platform linux/amd64 \
		-v "$(PWD)":/usr/src/myapp \
		-w /usr/src/myapp \
		golang:1.25 \
		go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 \
		-ldflags "$(LDFLAGS)" \
		./cmd/marmot
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 (version: $(VERSION))"

build-darwin: version
	@echo "Building $(BINARY_NAME) for macOS..."
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 -ldflags "$(LDFLAGS)" ./cmd/marmot
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 -ldflags "$(LDFLAGS)" ./cmd/marmot
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 (version: $(VERSION))"
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 (version: $(VERSION))"

build-windows: version
	@echo "Building $(BINARY_NAME) for Windows..."
	@GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe -ldflags "$(LDFLAGS)" ./cmd/marmot
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe (version: $(VERSION))"

test:
	@echo "Running tests..."
	@go test -v -coverprofile=coverage.out ./...

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linter..."
	@go vet ./...
	@golangci-lint run || echo "golangci-lint not installed, skipping..."

clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME) || echo "Installation requires sudo or manual copy"

all: clean build-linux build-darwin build-windows
	@echo "All builds complete!"
	@echo "Versions: $(VERSION)"

deploy-linux: build-linux-docker
	@echo "Deploying $(BINARY_NAME) to server..."
	@scp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 evalumnos@148.202.89.77:/tmp/marmot
	@echo "Binary deployed to /tmp/marmot on server"
	@echo "Run: ssh evalumnos@148.202.89.77 'sudo cp /tmp/marmot /usr/local/bin/marmot && sudo chmod +x /usr/local/bin/marmot'"
