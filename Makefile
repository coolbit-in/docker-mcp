.PHONY: build test clean docker-build docker-push help

# Version Info
VERSION ?= 1.0.0
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v$(VERSION)")

# Go related variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
BINARY_NAME := docker-mcp
LDFLAGS := -ldflags "-X main.Version=$(GIT_TAG) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)"

# Docker related variables
DOCKER_IMAGE_NAME := coolbit-in/docker-mcp
DOCKER_IMAGE_TAG := $(shell echo $(GIT_TAG) | sed 's/^v//')

# Proxy settings (use environment variables if not set)
HTTP_PROXY ?= $(shell echo $$HTTP_PROXY)
HTTPS_PROXY ?= $(shell echo $$HTTPS_PROXY)
NO_PROXY ?= $(shell echo $$NO_PROXY)

# Operating systems and architectures
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

help:
	@echo "Docker MCP build tool"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Compile the binary for the current platform"
	@echo "  make all           Compile the binary for all supported platforms"
	@echo "  make test          Run tests"
	@echo "  make clean         Clean build artifacts"
	@echo "  make docker-build  Build Docker image"
	@echo "  make docker-push   Push Docker image to repository"
	@echo ""
	@echo "Options:"
	@echo "  HTTP_PROXY=http://proxy:port  Set HTTP proxy for Docker build"
	@echo "  HTTPS_PROXY=http://proxy:port Set HTTPS proxy for Docker build"
	@echo "  NO_PROXY=localhost,127.0.0.1  Set NO_PROXY for Docker build"
	@echo ""
	@echo "Version: $(GIT_TAG)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/docker-mcp

all: clean
	mkdir -p build
	$(foreach platform, $(PLATFORMS), \
		$(eval os := $(word 1, $(subst /, ,$(platform)))) \
		$(eval arch := $(word 2, $(subst /, ,$(platform)))) \
		GOOS=$(os) GOARCH=$(arch) $(GOBUILD) $(LDFLAGS) -o build/$(BINARY_NAME)_$(os)_$(arch) ./cmd/docker-mcp; \
	)
	@echo "Compiled binaries for all supported platforms in build/"

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf build/

docker-build:
	@echo "Building Docker image with the following settings:"
	@echo "  Image: $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"
	@echo "  Version: $(GIT_TAG)"
	@echo "  Build Date: $(BUILD_DATE)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@if [ -n "$(HTTP_PROXY)" ]; then echo "  HTTP_PROXY: $(HTTP_PROXY)"; fi
	@if [ -n "$(HTTPS_PROXY)" ]; then echo "  HTTPS_PROXY: $(HTTPS_PROXY)"; fi
	@if [ -n "$(NO_PROXY)" ]; then echo "  NO_PROXY: $(NO_PROXY)"; fi
	@echo ""
	
	docker build --progress=plain -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
		--build-arg VERSION=$(GIT_TAG) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		$(if $(HTTP_PROXY),--build-arg HTTP_PROXY=$(HTTP_PROXY),) \
		$(if $(HTTPS_PROXY),--build-arg HTTPS_PROXY=$(HTTPS_PROXY),) \
		$(if $(NO_PROXY),--build-arg NO_PROXY=$(NO_PROXY),) \
		--no-cache \
		.
	@echo "✅ Image built successfully: $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)"
	docker tag $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) $(DOCKER_IMAGE_NAME):latest
	@echo "✅ Image tagged successfully: $(DOCKER_IMAGE_NAME):latest"

docker-push:
	docker push $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_IMAGE_NAME):latest 