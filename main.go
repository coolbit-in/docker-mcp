package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
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
	Ports   []Port   `json:"ports"`
}

// Port represents a container port mapping
type Port struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort uint16 `json:"private_port"`
	PublicPort  uint16 `json:"public_port,omitempty"`
	Type        string `json:"type"`
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

// ContainerConfig represents container creation configuration
type ContainerConfig struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Command       []string          `json:"command,omitempty"`
	Env           []string          `json:"env,omitempty"`
	Ports         map[string]string `json:"ports,omitempty"`
	Volumes       []string          `json:"volumes,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	NetworkMode   string            `json:"network_mode,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
	AutoRemove    bool              `json:"auto_remove,omitempty"`
}

// ContainerCreatedResponse represents container creation response
type ContainerCreatedResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ContainerActionResponse represents container action response
type ContainerActionResponse struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Status string `json:"status"`
}

// ImageRemovedResponse represents image removal response
type ImageRemovedResponse struct {
	Removed     bool     `json:"removed"`
	ImageID     string   `json:"image_id"`
	UntaggedIDs []string `json:"untagged_ids,omitempty"`
}

// LogsResponse represents container logs response
type LogsResponse struct {
	ContainerID string `json:"container_id"`
	Logs        string `json:"logs"`
}

// BuildImageResponse represents image build response
type BuildImageResponse struct {
	Success bool     `json:"success"`
	ImageID string   `json:"image_id,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// CommandResponse represents the response from a command execution
type CommandResponse struct {
	ContainerID string `json:"container_id"`
	Command     string `json:"command"`
	Output      string `json:"output"`
}

// InspectResponse represents detailed container/image inspection result
type InspectResponse struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Details json.RawMessage `json:"details"`
}

// PullProgressResponse represents the progress of a pull operation
type PullProgressResponse struct {
	ImageName string `json:"image_name"`
	Status    string `json:"status"`
	Complete  bool   `json:"complete"`
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

	// List containers tool
	srv.AddTool(
		mcp.NewTool("list_containers",
			mcp.WithDescription("List all running Docker containers with their IDs, names, images and status. Returns array of container objects."),
			mcp.WithBoolean("all",
				mcp.Description("Show all containers (default shows just running)"),
				mcp.DefaultBool(false),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := s.listContainersHandler(ctx, request.Params.Arguments)
			if err != nil {
				return s.formatErrorResponse(err)
			}
			return s.formatResponse(result)
		},
	)

	// Execute command in container tool
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

	// Pull image tool
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

	// List images tool
	srv.AddTool(
		mcp.NewTool("list_images",
			mcp.WithDescription("List all locally stored Docker images. Returns array of image objects with ID, tags, size and creation time."),
			mcp.WithBoolean("all",
				mcp.Description("Show all images (default hides intermediate images)"),
				mcp.DefaultBool(false),
			),
		),
		s.listImagesHandler,
	)

	// Search Docker Hub tool
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

	// Create container tool
	srv.AddTool(
		mcp.NewTool("create_container",
			mcp.WithDescription("Create a new Docker container from an image. Requires image_name and container configuration."),
			mcp.WithString("image",
				mcp.Description("Image name to create container from"),
				mcp.Required(),
			),
			mcp.WithString("name",
				mcp.Description("Name for the container"),
				mcp.Required(),
			),
			mcp.WithArray("command",
				mcp.Description("Command to run in the container"),
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
				mcp.Description("Working directory inside the container"),
			),
			mcp.WithString("network_mode",
				mcp.Description("Network mode (bridge, host, none, container:<name|id>)"),
			),
			mcp.WithString("restart_policy",
				mcp.Description("Restart policy (no, always, on-failure, unless-stopped)"),
			),
			mcp.WithBoolean("auto_remove",
				mcp.Description("Automatically remove the container when it exits"),
				mcp.DefaultBool(false),
			),
		),
		s.createContainerHandler,
	)

	// Start container tool
	srv.AddTool(
		mcp.NewTool("start_container",
			mcp.WithDescription("Start one or more stopped containers."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to start"),
				mcp.Required(),
			),
		),
		s.startContainerHandler,
	)

	// Stop container tool
	srv.AddTool(
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
		s.stopContainerHandler,
	)

	// Restart container tool
	srv.AddTool(
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
		s.restartContainerHandler,
	)

	// Remove container tool
	srv.AddTool(
		mcp.NewTool("remove_container",
			mcp.WithDescription("Remove one or more containers."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to remove"),
				mcp.Required(),
			),
			mcp.WithBoolean("force",
				mcp.Description("Force the removal of a running container"),
				mcp.DefaultBool(false),
			),
			mcp.WithBoolean("volumes",
				mcp.Description("Remove anonymous volumes associated with the container"),
				mcp.DefaultBool(false),
			),
		),
		s.removeContainerHandler,
	)

	// Remove image tool
	srv.AddTool(
		mcp.NewTool("remove_image",
			mcp.WithDescription("Remove one or more images."),
			mcp.WithString("image",
				mcp.Description("Image ID or name to remove"),
				mcp.Required(),
			),
			mcp.WithBoolean("force",
				mcp.Description("Force removal of the image"),
				mcp.DefaultBool(false),
			),
		),
		s.removeImageHandler,
	)

	// Container logs tool
	srv.AddTool(
		mcp.NewTool("logs",
			mcp.WithDescription("Fetch the logs of a container."),
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
			mcp.WithNumber("tail",
				mcp.Description("Number of lines to show from the end of the logs"),
				mcp.DefaultNumber(100),
			),
		),
		s.containerLogsHandler,
	)

	// Inspect container tool
	srv.AddTool(
		mcp.NewTool("inspect_container",
			mcp.WithDescription("Return low-level information on Docker container."),
			mcp.WithString("container_id",
				mcp.Description("Container ID or name to inspect"),
				mcp.Required(),
			),
		),
		s.inspectContainerHandler,
	)

	// Inspect image tool
	srv.AddTool(
		mcp.NewTool("inspect_image",
			mcp.WithDescription("Return low-level information on Docker image."),
			mcp.WithString("image",
				mcp.Description("Image ID or name to inspect"),
				mcp.Required(),
			),
		),
		s.inspectImageHandler,
	)

	// Build image tool
	srv.AddTool(
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
				mcp.Description("Name and optionally a tag in the 'name:tag' format"),
				mcp.Required(),
			),
			mcp.WithBoolean("no_cache",
				mcp.Description("Do not use cache when building the image"),
				mcp.DefaultBool(false),
			),
			mcp.WithBoolean("pull",
				mcp.Description("Always attempt to pull a newer version of the image"),
				mcp.DefaultBool(false),
			),
		),
		s.buildImageHandler,
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
	params := request.Params.Arguments

	// Get optional all parameter
	all := false
	if allVal, ok := params["all"].(bool); ok {
		all = allVal
	}

	images, err := s.cli.ImageList(ctx, image.ListOptions{
		All: all,
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
	params, ok := args.(map[string]interface{})
	if !ok {
		params = make(map[string]interface{})
	}

	// Get optional all parameter
	all := false
	if allVal, ok := params["all"].(bool); ok {
		all = allVal
	}

	containers, err := s.cli.ContainerList(ctx, container.ListOptions{
		All: all,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var result []ContainerInfo
	for _, c := range containers {
		containerInfo := ContainerInfo{
			ID:      c.ID,
			Names:   c.Names,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
			Ports:   []Port{},
		}

		// Convert port mappings
		for _, p := range c.Ports {
			containerInfo.Ports = append(containerInfo.Ports, Port{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		result = append(result, containerInfo)
	}

	return result, nil
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

// createContainerHandler handles container creation requests
func (s *DockerMCPServer) createContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	// Extract required parameters
	imageName, ok := params["image"].(string)
	if !ok || imageName == "" {
		return s.formatErrorResponse(fmt.Errorf("image is required"))
	}

	containerName, ok := params["name"].(string)
	if !ok || containerName == "" {
		return s.formatErrorResponse(fmt.Errorf("name is required"))
	}

	// Create container configuration
	config := &container.Config{
		Image: imageName,
	}

	// Optional command
	if cmdArray, ok := params["command"].([]interface{}); ok && len(cmdArray) > 0 {
		cmd := make([]string, len(cmdArray))
		for i, c := range cmdArray {
			if s, ok := c.(string); ok {
				cmd[i] = s
			}
		}
		config.Cmd = cmd
	}

	// Optional environment variables
	if envArray, ok := params["env"].([]interface{}); ok && len(envArray) > 0 {
		env := make([]string, len(envArray))
		for i, e := range envArray {
			if s, ok := e.(string); ok {
				env[i] = s
			}
		}
		config.Env = env
	}

	// Optional working directory
	if workingDir, ok := params["working_dir"].(string); ok && workingDir != "" {
		config.WorkingDir = workingDir
	}

	// Host configuration
	hostConfig := &container.HostConfig{}

	// Optional port mappings
	if portMapObj, ok := params["ports"].(map[string]interface{}); ok && len(portMapObj) > 0 {
		portBindings := nat.PortMap{}
		exposedPorts := nat.PortSet{}

		for portMapping := range portMapObj {
			parts := strings.Split(portMapping, ":")
			if len(parts) != 2 {
				continue
			}

			hostPort := parts[0]
			containerPortProto := parts[1]

			containerPort, err := nat.NewPort(
				strings.Split(containerPortProto, "/")[1],
				strings.Split(containerPortProto, "/")[0],
			)
			if err != nil {
				continue
			}

			portBindings[containerPort] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			}

			exposedPorts[containerPort] = struct{}{}
		}

		hostConfig.PortBindings = portBindings
		config.ExposedPorts = exposedPorts
	}

	// Optional volume mappings
	if volumesArray, ok := params["volumes"].([]interface{}); ok && len(volumesArray) > 0 {
		volumes := make([]string, len(volumesArray))
		for i, v := range volumesArray {
			if s, ok := v.(string); ok {
				volumes[i] = s
			}
		}
		hostConfig.Binds = volumes
	}

	// Optional network mode
	if networkMode, ok := params["network_mode"].(string); ok && networkMode != "" {
		hostConfig.NetworkMode = container.NetworkMode(networkMode)
	}

	// Optional restart policy
	if restartPolicy, ok := params["restart_policy"].(string); ok && restartPolicy != "" {
		switch restartPolicy {
		case "no":
			hostConfig.RestartPolicy = container.RestartPolicy{Name: "no"}
		case "always":
			hostConfig.RestartPolicy = container.RestartPolicy{Name: "always"}
		case "unless-stopped":
			hostConfig.RestartPolicy = container.RestartPolicy{Name: "unless-stopped"}
		case "on-failure":
			hostConfig.RestartPolicy = container.RestartPolicy{Name: "on-failure", MaximumRetryCount: 3}
		}
	}

	// Optional auto-remove
	if autoRemove, ok := params["auto_remove"].(bool); ok {
		hostConfig.AutoRemove = autoRemove
	}

	// Create the container
	resp, err := s.cli.ContainerCreate(
		ctx,
		config,
		hostConfig,
		&network.NetworkingConfig{},
		nil,
		containerName,
	)

	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to create container: %w", err))
	}

	return s.formatResponse(ContainerCreatedResponse{
		ID:   resp.ID,
		Name: containerName,
	})
}

// startContainerHandler handles container start requests
func (s *DockerMCPServer) startContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	err := s.cli.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to start container: %w", err))
	}

	return s.formatResponse(ContainerActionResponse{
		ID:     containerID,
		Action: "start",
		Status: "success",
	})
}

// stopContainerHandler handles container stop requests
func (s *DockerMCPServer) stopContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	var timeoutSecs int = 10
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeoutSecs = int(timeoutVal)
	}

	err := s.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeoutSecs})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to stop container: %w", err))
	}

	return s.formatResponse(ContainerActionResponse{
		ID:     containerID,
		Action: "stop",
		Status: "success",
	})
}

// restartContainerHandler handles container restart requests
func (s *DockerMCPServer) restartContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	var timeoutSecs int = 10
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeoutSecs = int(timeoutVal)
	}

	err := s.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeoutSecs})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to restart container: %w", err))
	}

	return s.formatResponse(ContainerActionResponse{
		ID:     containerID,
		Action: "restart",
		Status: "success",
	})
}

// removeContainerHandler handles container removal requests
func (s *DockerMCPServer) removeContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	force := false
	if forceVal, ok := params["force"].(bool); ok {
		force = forceVal
	}

	removeVolumes := false
	if volumesVal, ok := params["volumes"].(bool); ok {
		removeVolumes = volumesVal
	}

	err := s.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         force,
		RemoveVolumes: removeVolumes,
	})

	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to remove container: %w", err))
	}

	return s.formatResponse(ContainerActionResponse{
		ID:     containerID,
		Action: "remove",
		Status: "success",
	})
}

// removeImageHandler handles image removal requests
func (s *DockerMCPServer) removeImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	imageID, ok := params["image"].(string)
	if !ok || imageID == "" {
		return s.formatErrorResponse(fmt.Errorf("image is required"))
	}

	force := false
	if forceVal, ok := params["force"].(bool); ok {
		force = forceVal
	}

	response, err := s.cli.ImageRemove(ctx, imageID, image.RemoveOptions{
		Force: force,
	})

	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to remove image: %w", err))
	}

	// Process removal response
	var result ImageRemovedResponse
	if len(response) > 0 {
		result.Removed = true
		result.ImageID = imageID

		// Check for untagged images
		for _, item := range response {
			if item.Untagged != "" {
				result.UntaggedIDs = append(result.UntaggedIDs, item.Untagged)
			}
		}
	}

	return s.formatResponse(result)
}

// containerLogsHandler handles container logs requests
func (s *DockerMCPServer) containerLogsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	follow := false
	if followVal, ok := params["follow"].(bool); ok {
		follow = followVal
	}

	timestamps := false
	if tsVal, ok := params["timestamps"].(bool); ok {
		timestamps = tsVal
	}

	tail := "100"
	if tailVal, ok := params["tail"].(float64); ok {
		if tailVal < 0 {
			tail = "all"
		} else {
			tail = fmt.Sprintf("%d", int(tailVal))
		}
	}

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: timestamps,
		Tail:       tail,
	}

	reader, err := s.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to get container logs: %w", err))
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to read logs: %w", err))
	}

	return s.formatResponse(LogsResponse{
		ContainerID: containerID,
		Logs:        string(logs),
	})
}

// inspectContainerHandler handles container inspection requests
func (s *DockerMCPServer) inspectContainerHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return s.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	containerInfo, err := s.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to inspect container: %w", err))
	}

	// Convert to JSON
	details, err := json.Marshal(containerInfo)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to marshal container info: %w", err))
	}

	return s.formatResponse(InspectResponse{
		ID:      containerID,
		Type:    "container",
		Details: details,
	})
}

// inspectImageHandler handles image inspection requests
func (s *DockerMCPServer) inspectImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	imageID, ok := params["image"].(string)
	if !ok || imageID == "" {
		return s.formatErrorResponse(fmt.Errorf("image is required"))
	}

	imageInfo, _, err := s.cli.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to inspect image: %w", err))
	}

	// Convert to JSON
	details, err := json.Marshal(imageInfo)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to marshal image info: %w", err))
	}

	return s.formatResponse(InspectResponse{
		ID:      imageID,
		Type:    "image",
		Details: details,
	})
}

// buildImageHandler handles image build requests
func (s *DockerMCPServer) buildImageHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	contextPath, ok := params["context_path"].(string)
	if !ok || contextPath == "" {
		return s.formatErrorResponse(fmt.Errorf("context_path is required"))
	}

	dockerfileName := "Dockerfile"
	if df, ok := params["dockerfile"].(string); ok && df != "" {
		dockerfileName = df
	}

	tag, ok := params["tag"].(string)
	if !ok || tag == "" {
		return s.formatErrorResponse(fmt.Errorf("tag is required"))
	}

	noCache := false
	if noCacheVal, ok := params["no_cache"].(bool); ok {
		noCache = noCacheVal
	}

	pull := false
	if pullVal, ok := params["pull"].(bool); ok {
		pull = pullVal
	}

	// Verify dockerfile exists
	dockerfilePath := filepath.Join(contextPath, dockerfileName)
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return s.formatErrorResponse(fmt.Errorf("dockerfile %s not found in context", dockerfileName))
	}

	// Create build context from directory
	buildContext, err := archive.TarWithOptions(contextPath, &archive.TarOptions{})
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to create build context: %w", err))
	}
	defer buildContext.Close()

	// Build options
	buildOptions := types.ImageBuildOptions{
		Dockerfile: dockerfileName,
		Tags:       []string{tag},
		NoCache:    noCache,
		PullParent: pull,
		Remove:     true,
	}

	// Execute build
	resp, err := s.cli.ImageBuild(ctx, buildContext, buildOptions)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to build image: %w", err))
	}
	defer resp.Body.Close()

	// Read build output
	buildOutput, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.formatErrorResponse(fmt.Errorf("failed to read build output: %w", err))
	}

	// Look for successfully built message
	outputStr := string(buildOutput)
	if !strings.Contains(outputStr, "Successfully built") {
		return s.formatResponse(BuildImageResponse{
			Success: false,
			Error:   "Build failed, check build output",
		})
	}

	// Extract image ID if available
	imageID := ""
	if idIndex := strings.Index(outputStr, "Successfully built "); idIndex > 0 {
		idPart := outputStr[idIndex+18:]
		if newlineIndex := strings.Index(idPart, "\n"); newlineIndex > 0 {
			imageID = strings.TrimSpace(idPart[:newlineIndex])
		}
	}

	return s.formatResponse(BuildImageResponse{
		Success: true,
		ImageID: imageID,
		Tags:    []string{tag},
	})
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
