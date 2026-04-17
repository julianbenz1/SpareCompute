package store

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

type Store struct {
	mu          sync.RWMutex
	nodes       map[string]common.Node
	deployments map[string]common.Deployment
	instances   map[string]common.Instance
	routes      map[string]common.ServiceRoute
	nextID      int64
}

func New() *Store {
	return &Store{
		nodes:       map[string]common.Node{},
		deployments: map[string]common.Deployment{},
		instances:   map[string]common.Instance{},
		routes:      map[string]common.ServiceRoute{},
	}
}

func (s *Store) NewID(prefix string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
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
	n.Labels = req.Labels
	n.TotalCPUCores = req.TotalCPUCores
	n.TotalRAMMB = req.TotalRAMMB
	n.TotalDiskMB = req.TotalDiskMB
	n.ReservedCPUPercent = req.ReservedCPUPercent
	n.ReservedRAMMB = req.ReservedRAMMB
	n.ReservedDiskMB = req.ReservedDiskMB
	n.LastSeenAt = time.Now().UTC()
	s.nodes[n.ID] = n
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
	return d, true
}

