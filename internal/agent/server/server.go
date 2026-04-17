package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/agent/runtime"
	"github.com/julianbenz1/SpareCompute/internal/common"
)

type Server struct {
	docker     *runtime.Docker
	sharedDir  string
	publicAddr string
}

func New(docker *runtime.Docker, sharedDir, publicAddr string) *Server {
	return &Server{
		docker:     docker,
		sharedDir:  sharedDir,
		publicAddr: publicAddr,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/runtime/start", s.handleStart)
	mux.HandleFunc("/api/runtime/stop", s.handleStop)
	mux.HandleFunc("/api/runtime/checkpoint", s.handleCheckpoint)
	mux.HandleFunc("/api/runtime/restore", s.handleRestore)
	mux.HandleFunc("/api/runtime/health", s.handleHealth)
	return mux
}

func (s *Server) PublicAddress() string {
	return s.publicAddr
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.RuntimeStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.InstanceID == "" || req.Image == "" || req.InternalPort <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instance_id, image, internal_port are required"})
		return
	}
	containerName := "sparecompute-" + req.InstanceID
	resp, err := s.docker.StartContainer(r.Context(), req.Image, containerName, req.Env, req.InternalPort)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.RuntimeStopRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.ContainerRef == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "container_ref is required"})
		return
	}
	if err := s.docker.StopContainer(r.Context(), req.ContainerRef); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleCheckpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.RuntimeCheckpointRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.ContainerRef == "" || req.MigrationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "container_ref and migration_id are required"})
		return
	}
	if err := s.docker.CheckpointContainer(r.Context(), req.ContainerRef, req.MigrationID, s.sharedDir); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "checkpointed"})
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var req common.RuntimeRestoreRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.InstanceID == "" || req.Image == "" || req.InternalPort <= 0 || req.MigrationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instance_id, image, internal_port, migration_id are required"})
		return
	}
	containerName := "sparecompute-" + req.InstanceID
	resp, err := s.docker.RestoreContainer(r.Context(), req.Image, containerName, req.Env, req.InternalPort, req.MigrationID, s.sharedDir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func NewHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

