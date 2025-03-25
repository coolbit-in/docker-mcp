package models

import (
	"encoding/json"
	"time"
)

// APIResponse represents a standardized API response structure
// Used for all API endpoints to ensure consistent response format
type APIResponse struct {
	Success   bool            `json:"success"`         // Indicates if the operation was successful
	Data      json.RawMessage `json:"data"`            // The actual response data
	Error     string          `json:"error,omitempty"` // Error message if operation failed
	Count     int             `json:"count,omitempty"` // Number of items in response
	Timestamp time.Time       `json:"timestamp"`       // Response timestamp
}

// ContainerInfo represents detailed information about a Docker container
type ContainerInfo struct {
	ID      string   `json:"id"`      // Container ID
	Names   []string `json:"names"`   // Container names
	Image   string   `json:"image"`   // Image name
	Status  string   `json:"status"`  // Container status (e.g., running, stopped)
	State   string   `json:"state"`   // Container state
	Created int64    `json:"created"` // Creation timestamp
	Ports   []Port   `json:"ports"`   // Port mappings
}

// Port represents a container port mapping configuration
type Port struct {
	IP          string `json:"ip,omitempty"`          // Host IP address
	PrivatePort uint16 `json:"private_port"`          // Container port
	PublicPort  uint16 `json:"public_port,omitempty"` // Host port
	Type        string `json:"type"`                  // Protocol type (tcp/udp)
}

// ImageInfo represents detailed information about a Docker image
type ImageInfo struct {
	ID         string   `json:"id"`         // Image ID
	Tags       []string `json:"tags"`       // Image tags
	Size       int64    `json:"size"`       // Image size in bytes
	Created    int64    `json:"created"`    // Creation timestamp
	Containers int64    `json:"containers"` // Number of containers using this image
}

// SearchResult represents a Docker Hub image search result
type SearchResult struct {
	Name        string `json:"name"`        // Image name
	Description string `json:"description"` // Image description
	Official    bool   `json:"official"`    // Whether it's an official image
	Automated   bool   `json:"automated"`   // Whether it's an automated build
	Stars       int    `json:"stars"`       // Number of stars
}

// ContainerConfig represents container creation configuration
type ContainerConfig struct {
	Name          string            `json:"name"`                     // Container name
	Image         string            `json:"image"`                    // Image to use
	Command       []string          `json:"command,omitempty"`        // Command to run
	Env           []string          `json:"env,omitempty"`            // Environment variables
	Ports         map[string]string `json:"ports,omitempty"`          // Port mappings
	Volumes       []string          `json:"volumes,omitempty"`        // Volume mappings
	WorkingDir    string            `json:"working_dir,omitempty"`    // Working directory
	NetworkMode   string            `json:"network_mode,omitempty"`   // Network mode
	RestartPolicy string            `json:"restart_policy,omitempty"` // Restart policy
	AutoRemove    bool              `json:"auto_remove,omitempty"`    // Auto-remove when stopped
}

// ContainerCreatedResponse represents the response after creating a container
type ContainerCreatedResponse struct {
	ID   string `json:"id"`   // Created container ID
	Name string `json:"name"` // Container name
}

// ContainerActionResponse represents the response for container operations
type ContainerActionResponse struct {
	ID     string `json:"id"`     // Container ID
	Action string `json:"action"` // Action performed
	Status string `json:"status"` // Operation status
}

// ImageRemovedResponse represents the response after removing an image
type ImageRemovedResponse struct {
	Removed     bool     `json:"removed"`                // Whether image was removed
	ImageID     string   `json:"image_id"`               // Removed image ID
	UntaggedIDs []string `json:"untagged_ids,omitempty"` // IDs of untagged images
}

// LogsResponse represents container logs response
type LogsResponse struct {
	ContainerID string `json:"container_id"` // Container ID
	Logs        string `json:"logs"`         // Container logs
}

// BuildImageResponse represents image build operation response
type BuildImageResponse struct {
	Success bool     `json:"success"`            // Build success status
	ImageID string   `json:"image_id,omitempty"` // Built image ID
	Tags    []string `json:"tags,omitempty"`     // Image tags
	Error   string   `json:"error,omitempty"`    // Error message if build failed
}

// CommandResponse represents command execution response
type CommandResponse struct {
	ContainerID string `json:"container_id"` // Container ID
	Command     string `json:"command"`      // Executed command
	Output      string `json:"output"`       // Command output
}

// InspectResponse represents detailed inspection response
type InspectResponse struct {
	ID      string          `json:"id"`      // Object ID
	Type    string          `json:"type"`    // Object type (container/image)
	Details json.RawMessage `json:"details"` // Detailed information
}

// PullProgressResponse represents image pull progress
type PullProgressResponse struct {
	ImageName string `json:"image_name"` // Image being pulled
	Status    string `json:"status"`     // Current status
	Complete  bool   `json:"complete"`   // Whether pull is complete
}

// ProgressEvent represents an image pull progress event
type ProgressEvent struct {
	Status         string `json:"status"` // Current status message
	ProgressDetail struct {
		Current int64 `json:"current"` // Current progress
		Total   int64 `json:"total"`   // Total size
	} `json:"progressDetail"`
	ID string `json:"id"` // Layer ID
}
