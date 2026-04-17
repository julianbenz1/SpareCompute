package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/common"
	_ "modernc.org/sqlite"
)

type Store struct {
	mu          sync.RWMutex
	nodes       map[string]common.Node
	deployments map[string]common.Deployment
	instances   map[string]common.Instance
	routes      map[string]common.ServiceRoute
	nextID      int64
	db          *sql.DB
}

func New() *Store {
	return &Store{
		nodes:       map[string]common.Node{},
		deployments: map[string]common.Deployment{},
		instances:   map[string]common.Instance{},
		routes:      map[string]common.ServiceRoute{},
	}
}

func NewSQLite(dbPath string) (*Store, error) {
	if strings.TrimSpace(dbPath) == "" {
		return New(), nil
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	st := New()
	st.db = db
	if err := st.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := st.loadFromDB(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) NewID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	s.persistMetaLocked("next_id", fmt.Sprintf("%d", s.nextID))
	return fmt.Sprintf("%s-%06d", prefix, s.nextID)
}

func (s *Store) UpsertNode(req common.NodeRegisterRequest) common.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[req.ID]
	if !ok {
		n = common.Node{
			ID:     req.ID,
			Status: common.NodeOnline,
		}
	}
	n.Name = req.Name
	n.ControlAPIURL = req.ControlAPIURL
	n.PublicAddress = req.PublicAddress
	n.Labels = req.Labels
	n.TotalCPUCores = req.TotalCPUCores
	n.TotalRAMMB = req.TotalRAMMB
	n.TotalDiskMB = req.TotalDiskMB
	n.ReservedCPUPercent = req.ReservedCPUPercent
	n.ReservedRAMMB = req.ReservedRAMMB
	n.ReservedDiskMB = req.ReservedDiskMB
	n.LastSeenAt = time.Now().UTC()
	s.nodes[n.ID] = n
	s.persistNodeLocked(n)
	return n
}

func (s *Store) UpdateHeartbeat(req common.NodeHeartbeatRequest) (common.Node, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.nodes[req.NodeID]
	if !ok {
		return common.Node{}, false
	}
	n.ShareableCPU = req.ShareableCPU
	n.ShareableRAMMB = req.ShareableRAMMB
	n.ShareableDiskMB = req.ShareableDisk
	n.MaintenanceMode = req.Maintenance
	n.Status = req.Status
	n.LastSeenAt = time.Now().UTC()
	s.nodes[n.ID] = n
	s.persistNodeLocked(n)
	return n, true
}

func (s *Store) ListNodes() []common.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) GetNode(id string) (common.Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.nodes[id]
	return n, ok
}

func (s *Store) SaveDeployment(d common.Deployment) common.Deployment {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[d.ID] = d
	s.persistDeploymentLocked(d)
	return d
}

func (s *Store) ListDeployments() []common.Deployment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.Deployment, 0, len(s.deployments))
	for _, d := range s.deployments {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func (s *Store) SaveInstance(i common.Instance) common.Instance {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.instances[i.ID] = i
	s.persistInstanceLocked(i)
	return i
}

func (s *Store) GetActiveInstanceByDeployment(depID string) (common.Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, i := range s.instances {
		if i.DeploymentID == depID && i.Status == common.InstanceRunning {
			return i, true
		}
	}
	return common.Instance{}, false
}

func (s *Store) SaveRoute(r common.ServiceRoute) common.ServiceRoute {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[r.DeploymentID] = r
	s.persistRouteLocked(r)
	return r
}

func (s *Store) ListRoutes() []common.ServiceRoute {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.ServiceRoute, 0, len(s.routes))
	for _, r := range s.routes {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeploymentID < out[j].DeploymentID })
	return out
}

func (s *Store) MarkDeploymentStatus(deploymentID string, status common.DeploymentStatus, activeNodeID string) (common.Deployment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deployments[deploymentID]
	if !ok {
		return common.Deployment{}, false
	}
	d.Status = status
	d.ActiveNodeID = activeNodeID
	d.UpdatedAt = time.Now().UTC()
	s.deployments[d.ID] = d
	s.persistDeploymentLocked(d)
	return d, true
}

func (s *Store) GetInstance(id string) (common.Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.instances[id]
	return i, ok
}

func (s *Store) ListInstances() []common.Instance {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.Instance, 0, len(s.instances))
	for _, i := range s.instances {
		out = append(out, i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (s *Store) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			control_api_url TEXT NOT NULL,
			public_address TEXT NOT NULL,
			status TEXT NOT NULL,
			labels_json TEXT NOT NULL,
			total_cpu_cores INTEGER NOT NULL,
			total_ram_mb INTEGER NOT NULL,
			total_disk_mb INTEGER NOT NULL,
			reserved_cpu_percent INTEGER NOT NULL,
			reserved_ram_mb INTEGER NOT NULL,
			reserved_disk_mb INTEGER NOT NULL,
			shareable_cpu INTEGER NOT NULL,
			shareable_ram_mb INTEGER NOT NULL,
			shareable_disk_mb INTEGER NOT NULL,
			maintenance_mode INTEGER NOT NULL,
			last_seen_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS deployments (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			image TEXT NOT NULL,
			cpu_limit_percent INTEGER NOT NULL,
			ram_limit_mb INTEGER NOT NULL,
			disk_limit_mb INTEGER NOT NULL,
			replicas INTEGER NOT NULL,
			priority TEXT NOT NULL,
			exposed_port INTEGER NOT NULL,
			internal_port INTEGER NOT NULL,
			domain TEXT NOT NULL,
			status TEXT NOT NULL,
			restart_policy TEXT NOT NULL,
			active_node_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS instances (
			id TEXT PRIMARY KEY,
			deployment_id TEXT NOT NULL,
			node_id TEXT NOT NULL,
			container_id TEXT NOT NULL,
			container_ref TEXT NOT NULL,
			status TEXT NOT NULL,
			health_status TEXT NOT NULL,
			internal_ip TEXT NOT NULL,
			internal_port INTEGER NOT NULL,
			host_port INTEGER NOT NULL,
			started_at TEXT NOT NULL,
			last_health_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS routes (
			deployment_id TEXT PRIMARY KEY,
			id TEXT NOT NULL,
			domain TEXT NOT NULL,
			active_instance_id TEXT NOT NULL,
			tls_enabled INTEGER NOT NULL,
			updated_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadFromDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var nextIDRaw string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key='next_id'`).Scan(&nextIDRaw)
	if err == nil {
		var parsed int64
		_, _ = fmt.Sscanf(nextIDRaw, "%d", &parsed)
		s.nextID = parsed
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	if err := s.loadNodesLocked(); err != nil {
		return err
	}
	if err := s.loadDeploymentsLocked(); err != nil {
		return err
	}
	if err := s.loadInstancesLocked(); err != nil {
		return err
	}
	if err := s.loadRoutesLocked(); err != nil {
		return err
	}
	return nil
}

func (s *Store) loadNodesLocked() error {
	rows, err := s.db.Query(`SELECT id,name,control_api_url,public_address,status,labels_json,total_cpu_cores,total_ram_mb,total_disk_mb,reserved_cpu_percent,reserved_ram_mb,reserved_disk_mb,shareable_cpu,shareable_ram_mb,shareable_disk_mb,maintenance_mode,last_seen_at FROM nodes`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var n common.Node
		var labelsRaw string
		var maintenance int
		var lastSeen string
		if err := rows.Scan(&n.ID, &n.Name, &n.ControlAPIURL, &n.PublicAddress, &n.Status, &labelsRaw, &n.TotalCPUCores, &n.TotalRAMMB, &n.TotalDiskMB, &n.ReservedCPUPercent, &n.ReservedRAMMB, &n.ReservedDiskMB, &n.ShareableCPU, &n.ShareableRAMMB, &n.ShareableDiskMB, &maintenance, &lastSeen); err != nil {
			return err
		}
		n.MaintenanceMode = maintenance == 1
		if labelsRaw != "" {
			_ = json.Unmarshal([]byte(labelsRaw), &n.Labels)
		}
		n.LastSeenAt = parseTime(lastSeen)
		s.nodes[n.ID] = n
	}
	return rows.Err()
}

func (s *Store) loadDeploymentsLocked() error {
	rows, err := s.db.Query(`SELECT id,project_id,name,image,cpu_limit_percent,ram_limit_mb,disk_limit_mb,replicas,priority,exposed_port,internal_port,domain,status,restart_policy,active_node_id,created_at,updated_at FROM deployments`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d common.Deployment
		var createdAt, updatedAt string
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Name, &d.Image, &d.CPULimit, &d.RAMLimitMB, &d.DiskLimitMB, &d.Replicas, &d.Priority, &d.ExposedPort, &d.InternalPort, &d.Domain, &d.Status, &d.RestartPolicy, &d.ActiveNodeID, &createdAt, &updatedAt); err != nil {
			return err
		}
		d.CreatedAt = parseTime(createdAt)
		d.UpdatedAt = parseTime(updatedAt)
		s.deployments[d.ID] = d
	}
	return rows.Err()
}

func (s *Store) loadInstancesLocked() error {
	rows, err := s.db.Query(`SELECT id,deployment_id,node_id,container_id,container_ref,status,health_status,internal_ip,internal_port,host_port,started_at,last_health_at FROM instances`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var i common.Instance
		var startedAt, lastHealthAt string
		if err := rows.Scan(&i.ID, &i.DeploymentID, &i.NodeID, &i.ContainerID, &i.ContainerRef, &i.Status, &i.HealthStatus, &i.InternalIP, &i.InternalPort, &i.HostPort, &startedAt, &lastHealthAt); err != nil {
			return err
		}
		i.StartedAt = parseTime(startedAt)
		i.LastHealthAt = parseTime(lastHealthAt)
		s.instances[i.ID] = i
	}
	return rows.Err()
}

func (s *Store) loadRoutesLocked() error {
	rows, err := s.db.Query(`SELECT id,deployment_id,domain,active_instance_id,tls_enabled,updated_at FROM routes`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var r common.ServiceRoute
		var tlsEnabled int
		var updatedAt string
		if err := rows.Scan(&r.ID, &r.DeploymentID, &r.Domain, &r.ActiveInstanceID, &tlsEnabled, &updatedAt); err != nil {
			return err
		}
		r.TLSEnabled = tlsEnabled == 1
		r.UpdatedAt = parseTime(updatedAt)
		s.routes[r.DeploymentID] = r
	}
	return rows.Err()
}

func (s *Store) persistNodeLocked(n common.Node) {
	if s.db == nil {
		return
	}
	labels := toJSON(n.Labels)
	_, err := s.db.Exec(`INSERT INTO nodes (id,name,control_api_url,public_address,status,labels_json,total_cpu_cores,total_ram_mb,total_disk_mb,reserved_cpu_percent,reserved_ram_mb,reserved_disk_mb,shareable_cpu,shareable_ram_mb,shareable_disk_mb,maintenance_mode,last_seen_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name,control_api_url=excluded.control_api_url,public_address=excluded.public_address,status=excluded.status,labels_json=excluded.labels_json,total_cpu_cores=excluded.total_cpu_cores,total_ram_mb=excluded.total_ram_mb,total_disk_mb=excluded.total_disk_mb,reserved_cpu_percent=excluded.reserved_cpu_percent,reserved_ram_mb=excluded.reserved_ram_mb,reserved_disk_mb=excluded.reserved_disk_mb,shareable_cpu=excluded.shareable_cpu,shareable_ram_mb=excluded.shareable_ram_mb,shareable_disk_mb=excluded.shareable_disk_mb,maintenance_mode=excluded.maintenance_mode,last_seen_at=excluded.last_seen_at`,
		n.ID, n.Name, n.ControlAPIURL, n.PublicAddress, n.Status, labels, n.TotalCPUCores, n.TotalRAMMB, n.TotalDiskMB, n.ReservedCPUPercent, n.ReservedRAMMB, n.ReservedDiskMB, n.ShareableCPU, n.ShareableRAMMB, n.ShareableDiskMB, boolToInt(n.MaintenanceMode), formatTime(n.LastSeenAt))
	logPersistErr("node", err)
}

func (s *Store) persistDeploymentLocked(d common.Deployment) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO deployments (id,project_id,name,image,cpu_limit_percent,ram_limit_mb,disk_limit_mb,replicas,priority,exposed_port,internal_port,domain,status,restart_policy,active_node_id,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET project_id=excluded.project_id,name=excluded.name,image=excluded.image,cpu_limit_percent=excluded.cpu_limit_percent,ram_limit_mb=excluded.ram_limit_mb,disk_limit_mb=excluded.disk_limit_mb,replicas=excluded.replicas,priority=excluded.priority,exposed_port=excluded.exposed_port,internal_port=excluded.internal_port,domain=excluded.domain,status=excluded.status,restart_policy=excluded.restart_policy,active_node_id=excluded.active_node_id,created_at=excluded.created_at,updated_at=excluded.updated_at`,
		d.ID, d.ProjectID, d.Name, d.Image, d.CPULimit, d.RAMLimitMB, d.DiskLimitMB, d.Replicas, d.Priority, d.ExposedPort, d.InternalPort, d.Domain, d.Status, d.RestartPolicy, d.ActiveNodeID, formatTime(d.CreatedAt), formatTime(d.UpdatedAt))
	logPersistErr("deployment", err)
}

func (s *Store) persistInstanceLocked(i common.Instance) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO instances (id,deployment_id,node_id,container_id,container_ref,status,health_status,internal_ip,internal_port,host_port,started_at,last_health_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET deployment_id=excluded.deployment_id,node_id=excluded.node_id,container_id=excluded.container_id,container_ref=excluded.container_ref,status=excluded.status,health_status=excluded.health_status,internal_ip=excluded.internal_ip,internal_port=excluded.internal_port,host_port=excluded.host_port,started_at=excluded.started_at,last_health_at=excluded.last_health_at`,
		i.ID, i.DeploymentID, i.NodeID, i.ContainerID, i.ContainerRef, i.Status, i.HealthStatus, i.InternalIP, i.InternalPort, i.HostPort, formatTime(i.StartedAt), formatTime(i.LastHealthAt))
	logPersistErr("instance", err)
}

func (s *Store) persistRouteLocked(r common.ServiceRoute) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO routes (id,deployment_id,domain,active_instance_id,tls_enabled,updated_at) VALUES (?,?,?,?,?,?)
ON CONFLICT(deployment_id) DO UPDATE SET id=excluded.id,domain=excluded.domain,active_instance_id=excluded.active_instance_id,tls_enabled=excluded.tls_enabled,updated_at=excluded.updated_at`,
		r.ID, r.DeploymentID, r.Domain, r.ActiveInstanceID, boolToInt(r.TLSEnabled), formatTime(r.UpdatedAt))
	logPersistErr("route", err)
}

func (s *Store) persistMetaLocked(key, value string) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	logPersistErr("meta", err)
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func logPersistErr(entity string, err error) {
	if err == nil {
		return
	}
	log.Printf("store persistence warning (%s): %v", entity, err)
}
