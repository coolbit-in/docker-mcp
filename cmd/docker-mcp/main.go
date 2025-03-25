package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/docker_mcp/pkg/server"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// Version information variables that can be set at build time
var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	Commit    = "unknown"
)

// CLI configuration options
var (
	dockerSocket string
	debug        bool
)

// initRootCmd initializes the root command with all its flags and subcommands
func initRootCmd() *cobra.Command {
	// Create the root command
	rootCmd := &cobra.Command{
		Use:     "docker-mcp",
		Short:   "Docker Management Control Panel",
		Long:    `Docker Management Control Panel (MCP) provides an interface to manage Docker containers, images, and networks.`,
		Version: Version,
		// RunE runs the actual command logic and returns an error if it fails
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCP()
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&dockerSocket, "docker-socket", "", "Docker socket path (e.g., 'unix:///var/run/docker.sock' or 'tcp://localhost:2375')")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode")

	// Add version flag that displays extended version information
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
Build Date: ` + BuildDate + `
Commit: ` + Commit + `
`)

	return rootCmd
}

// runMCP is the main function that starts the Docker MCP server
func runMCP() error {
	// Create Docker MCP server with the specified socket path
	dockerMCP, err := server.NewDockerMCPServer(dockerSocket)
	if err != nil {
		return fmt.Errorf("failed to create Docker MCP server: %w", err)
	}

	// Log startup information
	if debug {
		fmt.Printf("Starting Docker MCP server (version %s)\n", Version)
		fmt.Printf("Using Docker socket: %s\n", dockerSocket)
	}

	// Start MCP server
	if err := mcpserver.ServeStdio(dockerMCP.GetMCPServer()); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// main is the entry point of the application
func main() {
	// Initialize and execute the root command
	rootCmd := initRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
