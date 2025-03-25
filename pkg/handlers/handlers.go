package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/mark3labs/docker_mcp/pkg/docker"
	"github.com/mark3labs/docker_mcp/pkg/models"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler represents a Docker MCP request handler
type Handler struct {
	dockerClient *docker.Client
	progressCh   chan models.ProgressEvent
}

// NewHandler creates and initializes a new handler
func NewHandler() (*Handler, error) {
	client, err := docker.NewClient()
	if err != nil {
		return nil, err
	}

	return &Handler{
		dockerClient: client,
		progressCh:   make(chan models.ProgressEvent, 100),
	}, nil
}

// formatResponse formats the response in standard JSON format
// It handles different types of data and includes metadata like count and timestamp
func (h *Handler) formatResponse(data interface{}) (*mcp.CallToolResult, error) {
	response := models.APIResponse{
		Success:   true,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize data: %w", err)
	}

	response.Data = jsonData

	// Add count for slice types
	switch v := data.(type) {
	case []models.ContainerInfo:
		response.Count = len(v)
	case []models.ImageInfo:
		response.Count = len(v)
	case []models.SearchResult:
		response.Count = len(v)
	case []interface{}:
		response.Count = len(v)
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize response: %w", err)
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

// formatErrorResponse formats error responses in a consistent way
func (h *Handler) formatErrorResponse(err error) (*mcp.CallToolResult, error) {
	response := models.APIResponse{
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

// HandleListContainers handles container listing requests
// Supports optional 'all' parameter to show all containers including stopped ones
func (h *Handler) HandleListContainers(ctx context.Context, args interface{}) (*mcp.CallToolResult, error) {
	params, ok := args.(map[string]interface{})
	if !ok {
		params = make(map[string]interface{})
	}

	// Get optional 'all' parameter
	all := false
	if allVal, ok := params["all"].(bool); ok {
		all = allVal
	}

	containers, err := h.dockerClient.ListContainers(ctx, all)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to list containers: %w", err))
	}

	var result []models.ContainerInfo
	for _, c := range containers {
		containerInfo := models.ContainerInfo{
			ID:      c.ID,
			Names:   c.Names,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Created: c.Created,
			Ports:   []models.Port{},
		}

		// Convert port mappings
		for _, p := range c.Ports {
			containerInfo.Ports = append(containerInfo.Ports, models.Port{
				IP:          p.IP,
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			})
		}

		result = append(result, containerInfo)
	}

	return h.formatResponse(result)
}

// HandleExecCommand handles command execution requests in containers
func (h *Handler) HandleExecCommand(ctx context.Context, args interface{}) (*mcp.CallToolResult, error) {
	params, ok := args.(map[string]interface{})
	if !ok {
		return h.formatErrorResponse(fmt.Errorf("invalid parameters"))
	}

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	command, ok := params["command"].(string)
	if !ok || command == "" {
		return h.formatErrorResponse(fmt.Errorf("command is required"))
	}

	output, err := h.dockerClient.ExecCommand(ctx, containerID, command)
	if err != nil {
		return h.formatErrorResponse(err)
	}

	return h.formatResponse(models.CommandResponse{
		ContainerID: containerID,
		Command:     command,
		Output:      output,
	})
}

// HandlePullImage handles image pull requests with progress tracking
func (h *Handler) HandlePullImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments
	imageName, ok := params["image_name"].(string)
	if !ok || imageName == "" {
		return h.formatErrorResponse(fmt.Errorf("image_name is required"))
	}

	// Call Docker API to pull image
	reader, err := h.dockerClient.PullImage(ctx, imageName)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to pull image: %w", err))
	}
	defer reader.Close()

	// Handle streaming response
	decoder := json.NewDecoder(reader)
	for {
		var event models.ProgressEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return h.formatErrorResponse(fmt.Errorf("failed to decode progress event: %w", err))
		}

		// Send progress event
		h.progressCh <- event
	}

	result := models.PullProgressResponse{
		ImageName: imageName,
		Status:    "success",
		Complete:  true,
	}

	return h.formatResponse(result)
}

// HandleListImages handles image listing requests
func (h *Handler) HandleListImages(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	// Get optional 'all' parameter
	all := false
	if allVal, ok := params["all"].(bool); ok {
		all = allVal
	}

	images, err := h.dockerClient.ListImages(ctx, all)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to list images: %w", err))
	}

	var result []models.ImageInfo
	for _, img := range images {
		result = append(result, models.ImageInfo{
			ID:         img.ID,
			Tags:       img.RepoTags,
			Size:       img.Size,
			Created:    img.Created,
			Containers: img.Containers,
		})
	}

	return h.formatResponse(result)
}

// HandleSearchImage handles image search requests on Docker Hub
func (h *Handler) HandleSearchImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments
	term, ok := params["term"].(string)
	if !ok || term == "" {
		return h.formatErrorResponse(fmt.Errorf("search term is required"))
	}

	// Get optional limit parameter
	limit := 25 // default value
	if limitVal, ok := params["limit"].(float64); ok {
		limit = int(limitVal)
	}

	// Call Docker API to search images
	searchResults, err := h.dockerClient.SearchImages(ctx, term, limit)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to search images: %w", err))
	}

	// Format results
	var result []models.SearchResult
	for _, item := range searchResults {
		result = append(result, models.SearchResult{
			Name:        item.Name,
			Description: item.Description,
			Official:    item.IsOfficial,
			Automated:   item.IsAutomated,
			Stars:       item.StarCount,
		})
	}

	return h.formatResponse(result)
}

// HandleCreateContainer handles container creation requests
func (h *Handler) HandleCreateContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	// Extract required parameters
	imageName, ok := params["image"].(string)
	if !ok || imageName == "" {
		return h.formatErrorResponse(fmt.Errorf("image is required"))
	}

	containerName, ok := params["name"].(string)
	if !ok || containerName == "" {
		return h.formatErrorResponse(fmt.Errorf("name is required"))
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

			// Ensure format is port/protocol
			portParts := strings.Split(containerPortProto, "/")
			if len(portParts) != 2 {
				// If no protocol is specified, default to TCP
				containerPortProto = containerPortProto + "/tcp"
				portParts = []string{containerPortProto, "tcp"}
			}

			containerPort, err := nat.NewPort(portParts[1], portParts[0])
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

	// Optional auto removal
	if autoRemove, ok := params["auto_remove"].(bool); ok {
		hostConfig.AutoRemove = autoRemove
	}

	// Create container
	resp, err := h.dockerClient.CreateContainer(ctx, config, hostConfig, containerName)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to create container: %w", err))
	}

	return h.formatResponse(models.ContainerCreatedResponse{
		ID:   resp.ID,
		Name: containerName,
	})
}

// HandleStartContainer handles container start requests
func (h *Handler) HandleStartContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	err := h.dockerClient.StartContainer(ctx, containerID)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to start container: %w", err))
	}

	return h.formatResponse(models.ContainerActionResponse{
		ID:     containerID,
		Action: "start",
		Status: "success",
	})
}

// HandleStopContainer handles container stop requests
func (h *Handler) HandleStopContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	var timeoutSecs int = 10
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeoutSecs = int(timeoutVal)
	}

	err := h.dockerClient.StopContainer(ctx, containerID, &timeoutSecs)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to stop container: %w", err))
	}

	return h.formatResponse(models.ContainerActionResponse{
		ID:     containerID,
		Action: "stop",
		Status: "success",
	})
}

// HandleRestartContainer handles container restart requests
func (h *Handler) HandleRestartContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	var timeoutSecs int = 10
	if timeoutVal, ok := params["timeout"].(float64); ok {
		timeoutSecs = int(timeoutVal)
	}

	err := h.dockerClient.RestartContainer(ctx, containerID, &timeoutSecs)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to restart container: %w", err))
	}

	return h.formatResponse(models.ContainerActionResponse{
		ID:     containerID,
		Action: "restart",
		Status: "success",
	})
}

// HandleRemoveContainer handles container removal requests
func (h *Handler) HandleRemoveContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	force := false
	if forceVal, ok := params["force"].(bool); ok {
		force = forceVal
	}

	removeVolumes := false
	if volumesVal, ok := params["volumes"].(bool); ok {
		removeVolumes = volumesVal
	}

	err := h.dockerClient.RemoveContainer(ctx, containerID, force, removeVolumes)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to remove container: %w", err))
	}

	return h.formatResponse(models.ContainerActionResponse{
		ID:     containerID,
		Action: "remove",
		Status: "success",
	})
}

// HandleRemoveImage handles image removal requests
func (h *Handler) HandleRemoveImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	imageID, ok := params["image"].(string)
	if !ok || imageID == "" {
		return h.formatErrorResponse(fmt.Errorf("image is required"))
	}

	force := false
	if forceVal, ok := params["force"].(bool); ok {
		force = forceVal
	}

	response, err := h.dockerClient.RemoveImage(ctx, imageID, force)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to remove image: %w", err))
	}

	// Handle removal response
	var result models.ImageRemovedResponse
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

	return h.formatResponse(result)
}

// HandleContainerLogs handles container logs requests
func (h *Handler) HandleContainerLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
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

	reader, err := h.dockerClient.ContainerLogs(ctx, containerID, follow, timestamps, tail)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to get container logs: %w", err))
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to read logs: %w", err))
	}

	return h.formatResponse(models.LogsResponse{
		ContainerID: containerID,
		Logs:        string(logs),
	})
}

// HandleInspectContainer handles container inspection requests
func (h *Handler) HandleInspectContainer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	containerID, ok := params["container_id"].(string)
	if !ok || containerID == "" {
		return h.formatErrorResponse(fmt.Errorf("container_id is required"))
	}

	containerInfo, err := h.dockerClient.InspectContainer(ctx, containerID)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to inspect container: %w", err))
	}

	// Convert to JSON
	details, err := json.Marshal(containerInfo)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to serialize container information: %w", err))
	}

	return h.formatResponse(models.InspectResponse{
		ID:      containerID,
		Type:    "container",
		Details: details,
	})
}

// HandleInspectImage handles image inspection requests
func (h *Handler) HandleInspectImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	imageID, ok := params["image"].(string)
	if !ok || imageID == "" {
		return h.formatErrorResponse(fmt.Errorf("image is required"))
	}

	imageInfo, err := h.dockerClient.InspectImage(ctx, imageID)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to inspect image: %w", err))
	}

	// Convert to JSON
	details, err := json.Marshal(imageInfo)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to serialize image information: %w", err))
	}

	return h.formatResponse(models.InspectResponse{
		ID:      imageID,
		Type:    "image",
		Details: details,
	})
}

// HandleBuildImage handles image build requests
func (h *Handler) HandleBuildImage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	params := request.Params.Arguments

	contextPath, ok := params["context_path"].(string)
	if !ok || contextPath == "" {
		return h.formatErrorResponse(fmt.Errorf("context_path is required"))
	}

	dockerfileName := "Dockerfile"
	if df, ok := params["dockerfile"].(string); ok && df != "" {
		dockerfileName = df
	}

	tag, ok := params["tag"].(string)
	if !ok || tag == "" {
		return h.formatErrorResponse(fmt.Errorf("tag is required"))
	}

	noCache := false
	if noCacheVal, ok := params["no_cache"].(bool); ok {
		noCache = noCacheVal
	}

	pull := false
	if pullVal, ok := params["pull"].(bool); ok {
		pull = pullVal
	}

	resp, err := h.dockerClient.BuildImage(ctx, contextPath, dockerfileName, []string{tag}, noCache, pull)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to build image: %w", err))
	}
	defer resp.Body.Close()

	// Read build output
	buildOutput, err := io.ReadAll(resp.Body)
	if err != nil {
		return h.formatErrorResponse(fmt.Errorf("failed to read build output: %w", err))
	}

	// Find successful build message
	outputStr := string(buildOutput)
	if !strings.Contains(outputStr, "Successfully built") {
		return h.formatResponse(models.BuildImageResponse{
			Success: false,
			Error:   "build failed, please check build output",
		})
	}

	// Extract image ID (if available)
	imageID := ""
	if idIndex := strings.Index(outputStr, "Successfully built "); idIndex > 0 {
		idPart := outputStr[idIndex+18:]
		if newlineIndex := strings.Index(idPart, "\n"); newlineIndex > 0 {
			imageID = strings.TrimSpace(idPart[:newlineIndex])
		}
	}

	return h.formatResponse(models.BuildImageResponse{
		Success: true,
		ImageID: imageID,
		Tags:    []string{tag},
	})
}
