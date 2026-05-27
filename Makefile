# raidforge — root build orchestration. The Go backend lives in backend/.
BACKEND := backend
BIN     := bin/raidforge
# VERSION stamps the binary; CI/Docker pass the short SHA via BUILD_REF.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test vet lint run tidy clean

# Build the backend binary into ./bin
build:
	go -C $(BACKEND) build -ldflags "-s -w -X main.version=$(VERSION)" -o $(CURDIR)/$(BIN) ./cmd/raidforge

# Run backend tests with the race detector
test:
	go -C $(BACKEND) test -race ./...

# Static analysis
vet:
	go -C $(BACKEND) vet ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run

# Build and run the server locally
run: build
	$(CURDIR)/$(BIN)

# Tidy module files
tidy:
	go -C $(BACKEND) mod tidy

# Remove build artifacts
clean:
	rm -rf bin
