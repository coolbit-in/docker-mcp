package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DockerMCPServer encapsulates Docker client and MCP server components
type DockerMCPServer struct {
	cli        *client.Client
	server     *server.MCPServer
	progressCh chan ProgressEvent // Channel for progress events
}

// API response struct for structured output
type APIResponse struct {
	Success   bool            `json:"success"`
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error,omitempty"`
	Count     int             `json:"count,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// ContainerInfo contains structured container information
type ContainerInfo struct {
	ID      string   `json:"id"`
	Names   []string `json:"names"`
	Image   string   `json:"image"`
	Status  string   `json:"status"`
	State   string   `json:"state"`
	Created int64    `json:"created"`
}

// ImageInfo contains structured image information
type ImageInfo struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Size       int64    `json:"size"`
	Created    int64    `json:"created"`
	Containers int64    `json:"containers"`
}

// SearchResult contains Docker Hub search result
type SearchResult struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Official    bool   `json:"official"`
	Automated   bool   `json:"automated"`
	Stars       int    `json:"stars"`
}

// NewDockerMCPServer creates and initializes Docker client connection
func NewDockerMCPServer() (*DockerMCPServer, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix:///Users/richard.liu2/.rd/docker.sock"),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	s := &DockerMCPServer{
		cli: cli,
	}

	if err := s.setupServer(); err != nil {
		return nil, err
	}

	return s, nil
}

// setupServer configures MCP server and registers tools
func (s *DockerMCPServer) setupServer() error {
	s.progressCh = make(chan ProgressEvent, 100) // Initialize progress channel

	srv := server.NewMCPServer(
		"docker-mcp",
		"极.1.0",
		server.WithToolCapabilities(true),
	)
	srv.AddTool(
		mcp.NewTool("list_containers",
			mcp.WithDescription("List all running Docker containers with their IDs, names, images and status. Returns array of container objects."),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := s.listContainersHandler(ctx, request.Params.Arguments)
			if err != nil {
				return s.formatErrorResponse(err)
			}
			return s.formatResponse(result)
		},
	)

	srv.AddTool(
		mcp.NewTool("exec_command",
			mcp.WithDescription("Execute shell command in a specified container. Requires container_id and command parameters. Returns command output."),
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
			result, err := s.execCommandHandler(ctx, request.Params.Arguments)
			if err != nil {
				return s.formatErrorResponse(err)
			}
			return s.formatResponse(result)
		},
	)

	srv.AddTool(
		mcp.NewTool("pull_image",
			mcp.WithDescription("Pull Docker image from registry. Requires image_name parameter (format: name:tag). Returns streaming progress updates."),
			mcp.WithString("image_name",
				mcp.Description("Image name with tag (string)"),
				mcp.Required(),
			),
		),
		s.pullImageHandler,
	)

	srv.AddTool(
		mcp.NewTool("list_images",
			mcp.WithDescription("List all locally stored Docker images. Returns array of image objects with ID, tags, size and creation time."),
		),
		s.listImagesHandler,
	)

	srv.AddTool(
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
		s.searchImageHandler,
	)

	s.server = srv
	return nil
}

// formatResponse creates a standardized JSON response
func (s *DockerMCPServer) formatResponse(data interface{}) (*mcp.CallToolResult, error) {
	response := APIResponse{
		Success:   true,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	response.Data = jsonData

	// Add count if it's a slice
	switch v := data.(type) {
	case []ContainerInfo:
		response.Count = len(v)
	case []ImageInfo:
		response.Count = len(v)
	case []SearchResult:
		response.Count = len(v)
	case []interface{}:
		response.Count = len(v)
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(responseJSON),
			},
		},
	}, nil
}

// formatErrorResponse creates a standardized error response
func (s *DockerMCPServer) formatErrorResponse(err error) (*mcp.CallToolResult, error) {
	response := APIResponse{
		Success:   false,
		Error:     err.Error(),
		Timestamp: time.Now(),
	}

	responseJSON, _ := json.MarshalIndent(response, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: string(responseJSON),
			},
		},
	}, nil
}

// ProgressEvent defines image pull progress events
type ProgressEvent struct {
	Status         string `json:"status"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	ID string `json:"id"`
}

// PullProgressResponse represents the progress of a pull operation
type PullProgressResponse struct {
	ImageName string `json:"image_name"`
	Status    string `json:"status"`
	Complete  bool   `json:"complete"`
}

// pullImageHandler handles Docker image pull requests
func (s *DockerMCPServer) pullImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments
	imageName, ok := params["image_name"].(string)
	if !ok || imageName == "" {
		return s.formatErrorResponse(fmt.Errorf("image_name is required"))
	}

	// Call Docker API to pull image
	reader, err := s.cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to pull image: %w", err))
	}
	defer reader.Close()

	// Process streaming response
	decoder := json.NewDecoder(reader)
	for {
		var event ProgressEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return s.formatErrorResponse(fmt.Errorf("failed to decode progress event: %w", err))
		}

		// Send progress event
		s.progressCh <- event
	}

	result := PullProgressResponse{
		ImageName: imageName,
		Status:    "success",
		Complete:  true,
	}

	return s.formatResponse(result)
}

// listImagesHandler handles Docker image listing requests
func (s *DockerMCPServer) listImagesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	images, err := s.cli.ImageList(ctx, image.ListOptions{
		All: false, // Only show top-level images
	})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to list images: %w", err))
	}

	var result []ImageInfo
	for _, img := range images {
		result = append(result, ImageInfo{
			ID:         img.ID,
			Tags:       img.RepoTags,
			Size:       img.Size,
			Created:    img.Created,
			Containers: img.Containers,
		})
	}
	log.Printf("Images: %v", result)

	return s.formatResponse(result)
}

// searchImageHandler handles Docker image search requests
func (s *DockerMCPServer) searchImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract search term from request
	params := request.Params.Arguments
	term, ok := params["term"].(string)
	if !ok || term == "" {
		return s.formatErrorResponse(fmt.Errorf("search term is required"))
	}

	// Get optional limit parameter
	limit := 25 // default value
	if limitVal, ok := params["limit"].(float64); ok {
		limit = int(limitVal)
	}

	// Call Docker API to search images
	searchResults, err := s.cli.ImageSearch(ctx, term, registry.SearchOptions{
		Limit: limit,
	})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to search images: %w", err))
	}

	// Format results
	var result []SearchResult
	for _, item := range searchResults {
		result = append(result, SearchResult{
			Name:        item.Name,
			Description: item.Description,
			Official:    item.IsOfficial,
			Automated:   item.IsAutomated,
			Stars:       item.StarCount,
		})
	}

	return s.formatResponse(result)
}

// listContainersHandler handles container listing requests
func (s *DockerMCPServer) listContainersHandler(ctx context.Context, args interface{}) (interface{}, error) {
	containers, err := s.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []ContainerInfo
	for _, container := range containers {
		result = append(result, ContainerInfo{
			ID:      container.ID,
			Names:   container.Names,
			Image:   container.Image,
			Status:  container.Status,
			State:   container.State,
			Created: container.Created,
		})
	}

	return result, nil
}

// CommandResponse represents the response from a command execution
type CommandResponse struct {
	ContainerID string `json:"container_id"`
	Command     string `json:"command"`
	Output      string `json:"output"`
}

// execCommandHandler executes commands in containers
func (s *DockerMCPServer) execCommandHandler(ctx context.Context, args interface{}) (interface{}, error) {
	params, ok := args.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid arguments")
	}

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return nil, fmt.Errorf("container_id is required")
	}

	command, ok := params["command"].(string)
	if !ok || command == "" {
		return nil, fmt.Errorf("command is required")
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := s.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := s.cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read output: %w", err)
	}

	return CommandResponse{
		ContainerID: containerID,
		Command:     command,
		Output:      string(output),
	}, nil
}

// main is the entry point to start MCP server
func main() {
	mcp_server, err := NewDockerMCPServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := server.ServeStdio(mcp_server.server); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
