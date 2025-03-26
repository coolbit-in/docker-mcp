# Docker MCP (Model Context Protocol)

![Docker MCP](./docs/images/logo.png)

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

### Using Pre-built Binaries (Recommended)

1. Download the latest release for your platform from [GitHub Releases](https://github.com/coolbit-in/docker-mcp/releases)
2. Extract the archive:
   ```bash
   # For Linux/macOS:
   tar xzf docker-mcp_*_*.tar.gz
   
   # For Windows:
   # Extract the zip file using Windows Explorer
   ```
3. Move the binary to a directory in your PATH:
   ```bash
   # Linux/macOS:
   sudo mv docker-mcp /usr/local/bin/
   chmod +x /usr/local/bin/docker-mcp
   
   # Windows:
   # Move docker-mcp.exe to a directory in your PATH
   ```

### Building from Source

If you prefer to build from source or need a specific version:

```bash
git clone https://github.com/coolbit-in/docker-mcp.git
cd docker-mcp
go build ./cmd/docker-mcp
```

## Usage

### Command Line Options

```