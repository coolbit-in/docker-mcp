# Docker MCP (Model Context Protocol)

![Docker MCP](./docs/images/logo.jpeg)

[![Go Version](https://img.shields.io/github/go-mod/go-version/coolbit-in/docker-mcp)](https://golang.org/)
[![License](https://img.shields.io/github/license/coolbit-in/docker-mcp)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/coolbit-in/docker-mcp)](https://goreportcard.com/report/github.com/coolbit-in/docker-mcp)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/coolbit-in/docker-mcp/pulls)

**Docker MCP** is a powerful tool that implements the Model Context Protocol (MCP) for Docker operations, enabling AI assistants to interact seamlessly with the Docker engine. It provides a unified JSON API interface for AI models to perform common Docker operations, including container lifecycle management, image operations, log retrieval, and more.

The Model Context Protocol (MCP) is an open protocol developed by Anthropic that enables AI systems to interact with various data sources and tools in a standardized way. By implementing MCP for Docker operations, this tool bridges the gap between AI models and Docker infrastructure management.

## Features

- **Container Management**: Create, start, stop, restart, and remove containers
- **Image Operations**: Pull, list, search, and remove Docker images
- **Container Inspection**: Get detailed information about containers
- **Log Access**: Retrieve container logs with various filtering options
- **Command Execution**: Execute commands inside running containers
- **Build Support**: Build Docker images from Dockerfiles
- **Flexible Configuration**: Customizable Docker socket connection

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/coolbit-in/docker-mcp.git
cd docker-mcp

# Build the project
go build -o docker-mcp cmd/docker-mcp/main.go

# Run the binary
./docker-mcp
```

### Using pre-built binaries

Visit our [Releases](https://github.com/coolbit-in/docker-mcp/releases) page to download pre-built binaries for your system.

### Using Docker

```bash
docker pull coolbit-in/docker-mcp:latest
docker run -v /var/run/docker.sock:/var/run/docker.sock coolbit-in/docker-mcp
```

### Building Docker Image

```bash
# Build without proxy
docker build -t docker-mcp:latest .

# Build with proxy (useful in regions with network restrictions)
docker build --build-arg HTTPS_PROXY=http://your-proxy:port \
             --build-arg HTTP_PROXY=http://your-proxy:port \
             -t docker-mcp:latest .
             
# Using Makefile to build with proxy
make docker-build HTTPS_PROXY=http://your-proxy:port HTTP_PROXY=http://your-proxy:port
```

## Usage

### Command Line Options

```