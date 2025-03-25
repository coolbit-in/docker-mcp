package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

// Client wraps the Docker client
type Client struct {
	dockerClient *client.Client
}

// NewClient creates and initializes a Docker client connection
func NewClient() (*Client, error) {
	// Get Docker socket path, first try standard path
	dockerSockPath := "/var/run/docker.sock"

	// Check Rancher Desktop path (MacOS only)
	rdSockPath := os.ExpandEnv("${HOME}/.rd/docker.sock")
	if _, err := os.Stat(rdSockPath); err == nil {
		dockerSockPath = rdSockPath
	}

	// Check Colima path (MacOS only)
	colimaSockPath := os.ExpandEnv("${HOME}/.colima/docker.sock")
	if _, err := os.Stat(colimaSockPath); err == nil {
		dockerSockPath = colimaSockPath
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(
		client.WithHost("unix://"+dockerSockPath),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{
		dockerClient: cli,
	}, nil
}

// ListContainers lists all containers
func (c *Client) ListContainers(ctx context.Context, all bool) ([]types.Container, error) {
	return c.dockerClient.ContainerList(ctx, container.ListOptions{
		All: all,
	})
}

// ExecCommand executes a command in a container
func (c *Client) ExecCommand(ctx context.Context, containerID string, cmd string) (string, error) {
	// Configure execution options
	execConfig := container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		AttachStdout: true,
		AttachStderr: true,
	}

	// Create exec instance
	execID, err := c.dockerClient.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to the exec instance to get output
	resp, err := c.dockerClient.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read all output from the command
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	return string(output), nil
}

// PullImage pulls a Docker image from registry
func (c *Client) PullImage(ctx context.Context, imageName string) (io.ReadCloser, error) {
	return c.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
}

// ListImages lists local Docker images
func (c *Client) ListImages(ctx context.Context, all bool) ([]image.Summary, error) {
	return c.dockerClient.ImageList(ctx, image.ListOptions{
		All: all,
	})
}

// SearchImages searches for images on Docker Hub
func (c *Client) SearchImages(ctx context.Context, term string, limit int) ([]registry.SearchResult, error) {
	return c.dockerClient.ImageSearch(ctx, term, registry.SearchOptions{
		Limit: limit,
	})
}

// CreateContainer creates a new container
func (c *Client) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, name string) (container.CreateResponse, error) {
	return c.dockerClient.ContainerCreate(
		ctx,
		config,
		hostConfig,
		&network.NetworkingConfig{},
		nil,
		name,
	)
}

// StartContainer starts a container
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
}

// StopContainer stops a running container
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout *int) error {
	return c.dockerClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: timeout})
}

// RestartContainer restarts a container
func (c *Client) RestartContainer(ctx context.Context, containerID string, timeout *int) error {
	return c.dockerClient.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: timeout})
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(ctx context.Context, containerID string, force, removeVolumes bool) error {
	return c.dockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         force,
		RemoveVolumes: removeVolumes,
	})
}

// RemoveImage removes a Docker image
func (c *Client) RemoveImage(ctx context.Context, imageID string, force bool) ([]image.DeleteResponse, error) {
	return c.dockerClient.ImageRemove(ctx, imageID, image.RemoveOptions{
		Force: force,
	})
}

// ContainerLogs retrieves logs from a container
func (c *Client) ContainerLogs(ctx context.Context, containerID string, follow, timestamps bool, tail string) (io.ReadCloser, error) {
	// Configure log options
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: timestamps,
		Tail:       tail,
	}
	return c.dockerClient.ContainerLogs(ctx, containerID, options)
}

// InspectContainer retrieves detailed information about a container
func (c *Client) InspectContainer(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return c.dockerClient.ContainerInspect(ctx, containerID)
}

// InspectImage retrieves detailed information about an image
func (c *Client) InspectImage(ctx context.Context, imageID string) (types.ImageInspect, error) {
	imageInfo, _, err := c.dockerClient.ImageInspectWithRaw(ctx, imageID)
	return imageInfo, err
}

// BuildImage builds a Docker image from a Dockerfile and context
func (c *Client) BuildImage(ctx context.Context, contextPath string, dockerfileName string, tags []string, noCache, pull bool) (types.ImageBuildResponse, error) {
	// Verify that the Dockerfile exists in the context
	dockerfilePath := filepath.Join(contextPath, dockerfileName)
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return types.ImageBuildResponse{}, fmt.Errorf("dockerfile %s not found in context", dockerfileName)
	}

	// Create build context from the directory
	buildContext, err := archive.TarWithOptions(contextPath, &archive.TarOptions{})
	if err != nil {
		return types.ImageBuildResponse{}, fmt.Errorf("failed to create build context: %w", err)
	}

	// Configure build options
	buildOptions := types.ImageBuildOptions{
		Dockerfile: dockerfileName,
		Tags:       tags,
		NoCache:    noCache,
		PullParent: pull,
		Remove:     true,
	}

	// Execute the build
	return c.dockerClient.ImageBuild(ctx, buildContext, buildOptions)
}
