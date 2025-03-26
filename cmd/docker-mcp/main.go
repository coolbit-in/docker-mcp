package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	dockermcp "github.com/coolbit-in/docker-mcp/pkg/server"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// Version information variables that can be set at build time
var (
	Version   = "0.1.0"
	BuildDate = "unknown"
	Commit    = "unknown"
)

var (
	dockerSocket string
	logFormat    string
	logLevel     string
	logFile      string
)

// initRootCmd initializes the root command with all its flags and subcommands
func initRootCmd() *cobra.Command {
	// Create the root command
	rootCmd := &cobra.Command{
		Use:     "docker-mcp",
		Short:   "Docker Model Context Protocol Server",
		Long:    `Docker Model Context Protocol (MCP) Server provides an interface for AI models to manage Docker containers, images, and networks.`,
		Version: Version,
		// PreRun configures logging before running the command
		PreRun: func(cmd *cobra.Command, args []string) {
			setupLogging()
		},
		// RunE runs the actual command logic and returns an error if it fails
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCP()
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&dockerSocket, "docker-socket", "", "Docker socket path")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format (text or json)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", getDefaultLogPath(), "Log file path")

	// Add version flag that displays extended version information
	rootCmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "version %s" .Version}}
Build Date: ` + BuildDate + `
Commit: ` + Commit + `
`)

	return rootCmd
}

// setupLogging configures the global logger based on command line flags
func setupLogging() {
	// Set log level
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		fmt.Fprintf(os.Stderr, "Invalid log level: %s\n", logLevel)
		os.Exit(1)
	}

	// Set log output
	var output = os.Stdout

	// Ensure log file path exists
	if logFile != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(logFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create log directory '%s': %v\n", logDir, err)
			fmt.Fprintf(os.Stderr, "Falling back to stdout for logging\n")
		} else {
			// Open log file
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to open log file '%s': %v\n", logFile, err)
				fmt.Fprintf(os.Stderr, "Falling back to stdout for logging\n")
			} else {
				output = f
				// Don't close the file as it will be used throughout the program's lifecycle
			}
		}
	}

	// Create and set global log handler
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if logFormat == "json" {
		handler = slog.NewJSONHandler(output, opts)
	} else {
		handler = slog.NewTextHandler(output, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// runMCP is the main function that starts the Docker MCP server
func runMCP() error {
	// Create Docker MCP server with the specified socket path
	dockerMCP, err := dockermcp.NewDockerMCPServer(dockerSocket)
	if err != nil {
		return fmt.Errorf("failed to create Docker MCP server: %w", err)
	}

	slog.Info("Starting Docker MCP server",
		"docker_socket", dockerSocket,
		"log_format", logFormat,
		"log_level", logLevel,
		"log_file", logFile,
	)

	// Start MCP server
	if err := server.ServeStdio(dockerMCP.GetMCPServer()); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// getDefaultLogPath returns the default path for log file
func getDefaultLogPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// If unable to get user home directory, use temporary directory
		return filepath.Join(os.TempDir(), "docker-mcp.log")
	}
	return filepath.Join(home, ".docker-mcp", "docker-mcp.log")
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
