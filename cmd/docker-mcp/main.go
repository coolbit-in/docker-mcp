package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/docker_mcp/pkg/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create Docker MCP server
	dockerMCP, err := server.NewDockerMCPServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Start MCP server
	if err := mcpserver.ServeStdio(dockerMCP.GetMCPServer()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
