package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/docker_mcp/pkg/handlers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DockerMCPServer encapsulates the Docker MCP server
type DockerMCPServer struct {
	mcpServer *server.MCPServer
	handler   *handlers.Handler
}

// NewDockerMCPServer creates and initializes a new Docker MCP server
func NewDockerMCPServer() (*DockerMCPServer, error) {
	handler, err := handlers.NewHandler()
	if err != nil {
		return nil, fmt.Errorf("failed to create handler: %w", err)
	}

	srv := server.NewMCPServer(
		"docker-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s := &DockerMCPServer{
		mcpServer: srv,
		handler:   handler,
	}

	if err := s.registerTools(); err != nil {
		return nil, err
	}

	return s, nil
}

// GetMCPServer returns the underlying MCP server instance
func (s *DockerMCPServer) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}

// registerTools registers all available tools
func (s *DockerMCPServer) registerTools() error {
	// List containers tool
	s.mcpServer.AddTool(
		mcp.NewTool("list_containers",
			mcp.WithDescription("List all running Docker containers with their IDs, names, images, and status. Returns array of container objects."),
			mcp.WithBoolean("all",
				mcp.Description("Show all containers (default shows just running)"),
				mcp.DefaultBool(false),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.handler.HandleListContainers(ctx, request.Params.Arguments)
		},
	)

	// Execute command in container tool
	s.mcpServer.AddTool(
		mcp.NewTool("exec_command",
			mcp.WithDescription("Execute a shell command in a specified container. Requires container_id and command parameters. Returns command output."),
			mcp.WithString("container_id",
				mcp.Description("Container ID (string)"),
				mcp.Required(),
			),
			mcp.WithString("command",
				mcp.Description("Command to execute (string)"),
				mcp.Required(),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return s.handler.HandleExecCommand(ctx, request.Params.Arguments)
		},
	)

	// Pull image tool
	s.mcpServer.AddTool(
		mcp.NewTool("pull_image",
			mcp.WithDescription("Pull Docker image from registry. Requires image_name parameter (format: name:tag). Returns streaming progress updates."),
			mcp.WithString("image_name",
				mcp.Description("Image name with tag (string)"),
				mcp.Required(),
			),
		),
		s.handler.HandlePullImage,
	)

	// List images tool
	s.mcpServer.AddTool(
		mcp.NewTool("list_images",
			mcp.WithDescription("List all locally stored Docker images. Returns array of image objects with ID, tags, size and creation time."),
			mcp.WithBoolean("all",
				mcp.Description("Show all images (default hides intermediate images)"),
				mcp.DefaultBool(false),
			),
		),
		s.handler.HandleListImages,
	)

	// Search Docker Hub tool
	s.mcpServer.AddTool(
		mcp.NewTool("search",
			mcp.WithDescription("Search for Docker images on Docker Hub. Returns array of image results including name, description, official status, and star count."),
			mcp.WithString("term",
				mcp.Description("Search term (string)"),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return (optional, default: 25)"),
				mcp.DefaultNumber(25),
				mcp.Min(1),
				mcp.Max(100),
			),
		),
		s.handler.HandleSearchImage,
	)

	// Create container tool
	s.mcpServer.AddTool(
		mcp.NewTool("create_container",
			mcp.WithDescription("Create a new Docker container from an image. Requires image name and container configuration."),
			mcp.WithString("image",
				mcp.Description("Image name to create container from"),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("Container name"),
				mcp.Required(),
			),
			mcp.WithArray("command",
				mcp.Description("Command to run in container"),
			),
			mcp.WithArray("env",
				mcp.Description("Environment variables (format: KEY=VALUE)"),
			),
			mcp.WithObject("ports",
				mcp.Description("Port mappings (format: {\"host_port:container_port/protocol\": {}}"),
			),
			mcp.WithArray("volumes",
				mcp.Description("Volume mappings (format: host_path:container_path)"),
			),
			mcp.WithString("working_dir",
				mcp.Description("Working directory inside container"),
			),
			mcp.WithString("network_mode",
				mcp.Description("Network mode (bridge, host, none, container:<name|id>)"),
			),
			mcp.WithString("restart_policy",
				mcp.Description("Restart policy (no, always, on-failure, unless-stopped)"),
			),
			mcp.WithBoolean("auto_remove",
				mcp.Description("Automatically remove container when it exits"),
				mcp.DefaultBool(false),
			),
		),
		s.handler.HandleCreateContainer,
	)

	// Start container tool
	s.mcpServer.AddTool(
		mcp.NewTool("start_container",
			mcp.WithDescription("Start one or more stopped containers."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to start"),
				mcp.Required(),
			),
		),
		s.handler.HandleStartContainer,
	)

	// Stop container tool
	s.mcpServer.AddTool(
		mcp.NewTool("stop_container",
			mcp.WithDescription("Stop a running container."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to stop"),
				mcp.Required(),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Seconds to wait before killing the container"),
				mcp.DefaultNumber(10),
			),
		),
		s.handler.HandleStopContainer,
	)

	// Restart container tool
	s.mcpServer.AddTool(
		mcp.NewTool("restart_container",
			mcp.WithDescription("Restart a container."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to restart"),
				mcp.Required(),
			),
			mcp.WithNumber("timeout",
				mcp.Description("Seconds to wait before killing the container"),
				mcp.DefaultNumber(10),
			),
		),
		s.handler.HandleRestartContainer,
	)

	// Remove container tool
	s.mcpServer.AddTool(
		mcp.NewTool("remove_container",
			mcp.WithDescription("Remove one or more containers."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to remove"),
				mcp.Required(),
			),
			mcp.WithBoolean("force",
				mcp.Description("Force remove running container"),
				mcp.DefaultBool(false),
			),
			mcp.WithBoolean("volumes",
				mcp.Description("Remove anonymous volumes associated with the container"),
				mcp.DefaultBool(false),
			),
		),
		s.handler.HandleRemoveContainer,
	)

	// Remove image tool
	s.mcpServer.AddTool(
		mcp.NewTool("remove_image",
			mcp.WithDescription("Remove one or more images."),
			mcp.WithString("image",
				mcp.Description("Image ID or name to remove"),
				mcp.Required(),
			),
			mcp.WithBoolean("force",
				mcp.Description("Force remove image"),
				mcp.DefaultBool(false),
			),
		),
		s.handler.HandleRemoveImage,
	)

	// Container logs tool
	s.mcpServer.AddTool(
		mcp.NewTool("logs",
			mcp.WithDescription("Get logs from a container."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to get logs from"),
				mcp.Required(),
			),
			mcp.WithBoolean("follow",
				mcp.Description("Follow log output"),
				mcp.DefaultBool(false),
			),
			mcp.WithBoolean("timestamps",
				mcp.Description("Show timestamps"),
				mcp.DefaultBool(false),
			),
			mcp.WithString("tail",
				mcp.Description("Number of lines to show from the end of the logs"),
				mcp.DefaultString("all"),
			),
		),
		s.handler.HandleContainerLogs,
	)

	// Inspect container tool
	s.mcpServer.AddTool(
		mcp.NewTool("inspect_container",
			mcp.WithDescription("Return detailed information about a container."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to inspect"),
				mcp.Required(),
			),
		),
		s.handler.HandleInspectContainer,
	)

	// Inspect image tool
	s.mcpServer.AddTool(
		mcp.NewTool("inspect_image",
			mcp.WithDescription("Return detailed information about an image."),
			mcp.WithString("image",
				mcp.Description("Image ID or name to inspect"),
				mcp.Required(),
			),
		),
		s.handler.HandleInspectImage,
	)

	// Build image tool
	s.mcpServer.AddTool(
		mcp.NewTool("build_image",
			mcp.WithDescription("Build an image from a Dockerfile."),
			mcp.WithString("context_path",
				mcp.Description("Path to the build context"),
				mcp.Required(),
			),
			mcp.WithString("dockerfile",
				mcp.Description("Name of the Dockerfile"),
				mcp.DefaultString("Dockerfile"),
			),
			mcp.WithString("tag",
				mcp.Description("Tag to apply to the built image"),
				mcp.Required(),
			),
			mcp.WithBoolean("no_cache",
				mcp.Description("Do not use cache when building the image"),
				mcp.DefaultBool(false),
			),
			mcp.WithBoolean("pull",
				mcp.Description("Always attempt to pull a newer version of parent images"),
				mcp.DefaultBool(false),
			),
		),
		s.handler.HandleBuildImage,
	)

	return nil
}
