package server

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/common"
	"github.com/julianbenz1/SpareCompute/internal/panel/ingress"
	panelruntime "github.com/julianbenz1/SpareCompute/internal/panel/runtime"
	"github.com/julianbenz1/SpareCompute/internal/panel/scheduler"
	"github.com/julianbenz1/SpareCompute/internal/panel/store"
)

//go:embed ui/*
var uiFS embed.FS

type Server struct {
	store         *store.Store
	panelToken    string
	runtimeClient *panelruntime.Client
	ingress       *ingress.TraefikFileManager
}

func New(st *store.Store, panelToken string, runtimeClient *panelruntime.Client, ingressManager *ingress.TraefikFileManager) *Server {
	return &Server{
		store:         st,
		panelToken:    panelToken,
		runtimeClient: runtimeClient,
		ingress:       ingressManager,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/nodes/register", s.auth(s.handleNodeRegister))
	mux.HandleFunc("/api/nodes/heartbeat", s.auth(s.handleNodeHeartbeat))
	mux.HandleFunc("/api/nodes", s.handleNodesList)
	mux.HandleFunc("/api/deployments", s.handleDeployments)
	mux.HandleFunc("/api/routes", s.handleRoutes)
	mux.HandleFunc("/api/reconcile", s.handleReconcile)
	uiSub, err := fs.Sub(uiFS, "ui")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(uiSub)))
	}
	return mux
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.panelToken == "" {
			next(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") || strings.TrimPrefix(authHeader, "Bearer ") != s.panelToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleNodeRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.NodeRegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.ID == "" || req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and name are required"})
		return
	}
	node := s.store.UpsertNode(req)
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) handleNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.NodeHeartbeatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.NodeID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
		return
	}
	if req.Status == "" {
		req.Status = common.NodeOnline
	}
	node, ok := s.store.UpdateHeartbeat(req)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not registered"})
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleNodesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, s.store.ListNodes())
}

func (s *Server) handleDeployments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.ListDeployments())
	case http.MethodPost:
		s.createDeployment(w, r)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) createDeployment(w http.ResponseWriter, r *http.Request) {
	var req common.CreateDeploymentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Name == "" || req.Image == "" || req.InternalPort <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, image, internal_port are required"})
		return
	}
	if req.Replicas <= 0 {
		req.Replicas = 1
	}
	if req.Priority == "" {
		req.Priority = common.PriorityNormal
	}
	dep := common.Deployment{
		ID:           s.store.NewID("dep"),
		ProjectID:    req.ProjectID,
		Name:         req.Name,
		Image:        req.Image,
		CPULimit:     req.CPULimit,
		RAMLimitMB:   req.RAMLimitMB,
		DiskLimitMB:  req.DiskLimitMB,
		Replicas:     req.Replicas,
		Priority:     req.Priority,
		InternalPort: req.InternalPort,
		Domain:       req.Domain,
		Status:       common.DeploymentPending,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	dep = s.store.SaveDeployment(dep)
	if err := s.placeDeployment(dep, ""); err != nil {
		dep.Status = common.DeploymentFailed
		dep.UpdatedAt = time.Now().UTC()
		s.store.SaveDeployment(dep)
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	d, _ := s.store.MarkDeploymentStatus(dep.ID, common.DeploymentRunning, dep.ActiveNodeID)
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, s.store.ListRoutes())
}

func (s *Server) handleReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	deployments := s.store.ListDeployments()
	migrated := 0
	for _, dep := range deployments {
		if dep.Status != common.DeploymentRunning || dep.ActiveNodeID == "" {
			continue
		}
		node, ok := s.store.GetNode(dep.ActiveNodeID)
		if !ok || !scheduler.NeedsMigration(node, dep) {
			continue
		}
		if err := s.placeDeployment(dep, node.ID); err != nil {
			s.store.MarkDeploymentStatus(dep.ID, common.DeploymentDegraded, dep.ActiveNodeID)
			continue
		}
		migrated++
	}
	writeJSON(w, http.StatusOK, map[string]int{"migrated": migrated})
}

func (s *Server) placeDeployment(dep common.Deployment, excludeNodeID string) error {
	nodes := s.store.ListNodes()
	target, err := scheduler.SelectNode(nodes, dep, excludeNodeID)
	if err != nil {
		if errors.Is(err, scheduler.ErrNoSuitableNode) {
			return err
		}
		return errors.New("scheduler failed")
	}
	old, hadOld := s.store.GetActiveInstanceByDeployment(dep.ID)
	var oldNode common.Node
	var hasOldNode bool
	if hadOld {
		oldNode, hasOldNode = s.store.GetNode(old.NodeID)
		old.Status = common.InstanceStopping
		s.store.SaveInstance(old)
		s.store.MarkDeploymentStatus(dep.ID, common.DeploymentMigrating, old.NodeID)
	}
	newInstanceID, startResp, err := s.startOrMigrateInstance(dep, target, old, hadOld, oldNode, hasOldNode)
	if err != nil {
		if hadOld {
			old.Status = common.InstanceRunning
			s.store.SaveInstance(old)
		}
		return err
	}
	newInstance := common.Instance{
		ID:           newInstanceID,
		DeploymentID: dep.ID,
		NodeID:       target.ID,
		ContainerID:  startResp.ContainerID,
		ContainerRef: startResp.ContainerRef,
		Status:       common.InstanceRunning,
		HealthStatus: common.HealthHealthy,
		InternalIP:   target.PublicAddress,
		InternalPort: dep.InternalPort,
		HostPort:     startResp.HostPort,
		StartedAt:    time.Now().UTC(),
		LastHealthAt: time.Now().UTC(),
	}
	s.store.SaveInstance(newInstance)

	route := common.ServiceRoute{
		ID:               s.store.NewID("route"),
		DeploymentID:     dep.ID,
		Domain:           dep.Domain,
		ActiveInstanceID: newInstance.ID,
		TLSEnabled:       dep.Domain != "",
		UpdatedAt:        time.Now().UTC(),
	}
	s.store.SaveRoute(route)
	s.store.MarkDeploymentStatus(dep.ID, common.DeploymentRunning, target.ID)
	s.syncIngress()
	if hadOld && hasOldNode && old.ContainerRef != "" {
		_ = s.runtimeClient.Stop(context.Background(), oldNode.ControlAPIURL, common.RuntimeStopRequest{
			ContainerRef: old.ContainerRef,
		})
		old.Status = common.InstanceStopped
		s.store.SaveInstance(old)
	}
	return nil
}

func (s *Server) startOrMigrateInstance(dep common.Deployment, target common.Node, old common.Instance, hadOld bool, oldNode common.Node, hasOldNode bool) (string, common.RuntimeStartResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	newInstanceID := s.store.NewID("inst")
	startReq := common.RuntimeStartRequest{
		InstanceID:   newInstanceID,
		Image:        dep.Image,
		InternalPort: dep.InternalPort,
	}
	if !hadOld {
		resp, err := s.runtimeClient.Start(ctx, target.ControlAPIURL, startReq)
		return newInstanceID, resp, err
	}
	if hasOldNode && old.ContainerRef != "" && oldNode.ControlAPIURL != "" && target.ControlAPIURL != "" {
		migrationID := s.store.NewID("mig")
		if err := s.runtimeClient.Checkpoint(ctx, oldNode.ControlAPIURL, common.RuntimeCheckpointRequest{
			ContainerRef: old.ContainerRef,
			MigrationID:  migrationID,
		}); err == nil {
			restoreResp, restoreErr := s.runtimeClient.Restore(ctx, target.ControlAPIURL, common.RuntimeRestoreRequest{
				InstanceID:   newInstanceID,
				Image:        dep.Image,
				InternalPort: dep.InternalPort,
				MigrationID:  migrationID,
			})
			if restoreErr == nil {
				return newInstanceID, restoreResp, nil
			}
		}
	}
	resp, err := s.runtimeClient.Start(ctx, target.ControlAPIURL, startReq)
	return newInstanceID, resp, err
}

func (s *Server) syncIngress() {
	if s.ingress == nil || !s.ingress.Enabled() {
		return
	}
	instancesSlice := s.store.ListInstances()
	instanceMap := make(map[string]common.Instance, len(instancesSlice))
	for _, inst := range instancesSlice {
		instanceMap[inst.ID] = inst
	}
	nodesSlice := s.store.ListNodes()
	nodeMap := make(map[string]common.Node, len(nodesSlice))
	for _, node := range nodesSlice {
		nodeMap[node.ID] = node
	}
	targets := ingress.BuildTargets(s.store.ListRoutes(), instanceMap, nodeMap)
	_ = s.ingress.Sync(targets)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}
