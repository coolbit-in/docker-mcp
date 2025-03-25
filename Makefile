.PHONY: build test clean docker-build docker-push help

# 版本信息
VERSION ?= 1.0.0
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v$(VERSION)")

# Go相关变量
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
BINARY_NAME := docker-mcp
LDFLAGS := -ldflags "-X main.Version=$(GIT_TAG) -X main.BuildDate=$(BUILD_DATE) -X main.GitCommit=$(GIT_COMMIT)"

# Docker相关变量
DOCKER_IMAGE_NAME := mark3labs/docker-mcp
DOCKER_IMAGE_TAG := $(shell echo $(GIT_TAG) | sed 's/^v//')

# 操作系统和架构
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

help:
	@echo "Docker MCP 构建工具"
	@echo ""
	@echo "用法:"
	@echo "  make build        编译当前平台的二进制文件"
	@echo "  make all          编译所有支持平台的二进制文件"
	@echo "  make test         运行测试"
	@echo "  make clean        清理构建产物"
	@echo "  make docker-build 构建Docker镜像"
	@echo "  make docker-push  推送Docker镜像到仓库"
	@echo ""
	@echo "版本: $(GIT_TAG)"
	@echo "提交: $(GIT_COMMIT)"
	@echo "构建日期: $(BUILD_DATE)"

build:
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) ./cmd/docker-mcp

all: clean
	mkdir -p build
	$(foreach platform, $(PLATFORMS), \
		$(eval os := $(word 1, $(subst /, ,$(platform)))) \
		$(eval arch := $(word 2, $(subst /, ,$(platform)))) \
		GOOS=$(os) GOARCH=$(arch) $(GOBUILD) $(LDFLAGS) -o build/$(BINARY_NAME)_$(os)_$(arch) ./cmd/docker-mcp; \
	)
	@echo "已编译所有平台的二进制文件到 build/ 目录"

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf build/

docker-build:
	docker build -t $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) \
		--build-arg VERSION=$(GIT_TAG) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		.
	docker tag $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG) $(DOCKER_IMAGE_NAME):latest

docker-push:
	docker push $(DOCKER_IMAGE_NAME):$(DOCKER_IMAGE_TAG)
	docker push $(DOCKER_IMAGE_NAME):latest 