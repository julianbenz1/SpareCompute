package common

import "time"

type Priority string

const (
	PriorityHigh       Priority = "high"
	PriorityNormal     Priority = "normal"
	PriorityBestEffort Priority = "best_effort"
)

type DeploymentStatus string

const (
	DeploymentPending   DeploymentStatus = "pending"
	DeploymentStarting  DeploymentStatus = "starting"
	DeploymentRunning   DeploymentStatus = "running"
	DeploymentDegraded  DeploymentStatus = "degraded"
	DeploymentMigrating DeploymentStatus = "migrating"
	DeploymentStopping  DeploymentStatus = "stopping"
	DeploymentStopped   DeploymentStatus = "stopped"
	DeploymentFailed    DeploymentStatus = "failed"
)

type NodeStatus string

const (
	NodeOnline      NodeStatus = "online"
	NodeOffline     NodeStatus = "offline"
	NodeMaintenance NodeStatus = "maintenance"
)

type Node struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ControlAPIURL      string            `json:"control_api_url,omitempty"`
	PublicAddress      string            `json:"public_address,omitempty"`
	OwnerID            string            `json:"owner_id,omitempty"`
	Status             NodeStatus        `json:"status"`
	Labels             map[string]string `json:"labels,omitempty"`
	TotalCPUCores      int               `json:"total_cpu_cores"`
	TotalRAMMB         int64             `json:"total_ram_mb"`
	TotalDiskMB        int64             `json:"total_disk_mb"`
	ReservedCPUPercent int               `json:"reserved_cpu_percent"`
	ReservedRAMMB      int64             `json:"reserved_ram_mb"`
	ReservedDiskMB     int64             `json:"reserved_disk_mb"`
	ShareableCPU       int               `json:"shareable_cpu_percent"`
	ShareableRAMMB     int64             `json:"shareable_ram_mb"`
	ShareableDiskMB    int64             `json:"shareable_disk_mb"`
	MaintenanceMode    bool              `json:"maintenance_mode"`
	LastSeenAt         time.Time         `json:"last_seen_at"`
}

type NodeRegisterRequest struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	ControlAPIURL      string            `json:"control_api_url,omitempty"`
	PublicAddress      string            `json:"public_address,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
	TotalCPUCores      int               `json:"total_cpu_cores"`
	TotalRAMMB         int64             `json:"total_ram_mb"`
	TotalDiskMB        int64             `json:"total_disk_mb"`
	ReservedCPUPercent int               `json:"reserved_cpu_percent"`
	ReservedRAMMB      int64             `json:"reserved_ram_mb"`
	ReservedDiskMB     int64             `json:"reserved_disk_mb"`
}

type NodeHeartbeatRequest struct {
	NodeID         string      `json:"node_id"`
	CPUUsagePct    int         `json:"cpu_usage_pct"`
	LoadAvg1       float64     `json:"load_avg_1"`
	AvailableRAMMB int64       `json:"available_ram_mb"`
	FreeDiskMB     int64       `json:"free_disk_mb"`
	ShareableCPU   int         `json:"shareable_cpu_percent"`
	ShareableRAMMB int64       `json:"shareable_ram_mb"`
	ShareableDisk  int64       `json:"shareable_disk_mb"`
	Status         NodeStatus  `json:"status"`
	Maintenance    bool        `json:"maintenance_mode"`
	SentAt         *time.Time  `json:"sent_at,omitempty"`
	ContainerState interface{} `json:"container_state,omitempty"`
}

type Deployment struct {
	ID            string           `json:"id"`
	ProjectID     string           `json:"project_id,omitempty"`
	Name          string           `json:"name"`
	Image         string           `json:"image"`
	CPULimit      int              `json:"cpu_limit_percent"`
	RAMLimitMB    int64            `json:"ram_limit_mb"`
	DiskLimitMB   int64            `json:"disk_limit_mb"`
	Replicas      int              `json:"replicas"`
	Priority      Priority         `json:"priority"`
	ExposedPort   int              `json:"exposed_port,omitempty"`
	InternalPort  int              `json:"internal_port"`
	Domain        string           `json:"domain,omitempty"`
	Status        DeploymentStatus `json:"status"`
	RestartPolicy string           `json:"restart_policy,omitempty"`
	ActiveNodeID  string           `json:"active_node_id,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type CreateDeploymentRequest struct {
	ProjectID    string   `json:"project_id,omitempty"`
	Name         string   `json:"name"`
	Image        string   `json:"image"`
	CPULimit     int      `json:"cpu_limit_percent"`
	RAMLimitMB   int64    `json:"ram_limit_mb"`
	DiskLimitMB  int64    `json:"disk_limit_mb"`
	Replicas     int      `json:"replicas"`
	Priority     Priority `json:"priority"`
	InternalPort int      `json:"internal_port"`
	Domain       string   `json:"domain,omitempty"`
}

type InstanceStatus string

const (
	InstanceStarting InstanceStatus = "starting"
	InstanceRunning  InstanceStatus = "running"
	InstanceStopping InstanceStatus = "stopping"
	InstanceStopped  InstanceStatus = "stopped"
	InstanceFailed   InstanceStatus = "failed"
)

type HealthStatus string

const (
	HealthUnknown   HealthStatus = "unknown"
	HealthHealthy   HealthStatus = "healthy"
	HealthUnhealthy HealthStatus = "unhealthy"
)

type Instance struct {
	ID           string         `json:"id"`
	DeploymentID string         `json:"deployment_id"`
	NodeID       string         `json:"node_id"`
	ContainerID  string         `json:"container_id,omitempty"`
	ContainerRef string         `json:"container_ref,omitempty"`
	Status       InstanceStatus `json:"status"`
	HealthStatus HealthStatus   `json:"health_status"`
	InternalIP   string         `json:"internal_ip,omitempty"`
	InternalPort int            `json:"internal_port"`
	HostPort     int            `json:"host_port,omitempty"`
	StartedAt    time.Time      `json:"started_at"`
	LastHealthAt time.Time      `json:"last_health_at"`
}

type ServiceRoute struct {
	ID               string    `json:"id"`
	DeploymentID     string    `json:"deployment_id"`
	Domain           string    `json:"domain"`
	ActiveInstanceID string    `json:"active_instance_id"`
	TLSEnabled       bool      `json:"tls_enabled"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type RuntimeStartRequest struct {
	InstanceID   string            `json:"instance_id"`
	Image        string            `json:"image"`
	InternalPort int               `json:"internal_port"`
	Env          map[string]string `json:"env,omitempty"`
}

type RuntimeStartResponse struct {
	ContainerID  string `json:"container_id"`
	ContainerRef string `json:"container_ref"`
	HostPort     int    `json:"host_port"`
}

type RuntimeStopRequest struct {
	ContainerRef string `json:"container_ref"`
}

type RuntimeCheckpointRequest struct {
	ContainerRef string `json:"container_ref"`
	MigrationID  string `json:"migration_id"`
}

type RuntimeRestoreRequest struct {
	InstanceID   string            `json:"instance_id"`
	Image        string            `json:"image"`
	InternalPort int               `json:"internal_port"`
	MigrationID  string            `json:"migration_id"`
	Env          map[string]string `json:"env,omitempty"`
}
