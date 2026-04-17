package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/agent/client"
	"github.com/julianbenz1/SpareCompute/internal/agent/metrics"
	agentserver "github.com/julianbenz1/SpareCompute/internal/agent/server"
	"github.com/julianbenz1/SpareCompute/internal/agent/runtime"
)

func main() {
	panelURL := getenv("PANEL_URL", "http://127.0.0.1:8080")
	panelToken := os.Getenv("PANEL_TOKEN")
	nodeID := getenv("NODE_ID", hostNameFallback())
	intervalSeconds := getenvInt("HEARTBEAT_INTERVAL_SECONDS", 5)
	agentAddr := getenv("AGENT_ADDR", ":18080")
	controlURL := getenv("AGENT_CONTROL_URL", "http://127.0.0.1"+agentAddr)
	publicAddr := getenv("NODE_PUBLIC_ADDRESS", "127.0.0.1")
	migrationSharedDir := getenv("MIGRATION_SHARED_DIR", "/var/lib/sparecompute/migration")

	reserve := metrics.ReserveConfig{
		CPUPercent: getenvInt("RESERVED_CPU_PERCENT", 20),
		RAMMB:      int64(getenvInt("RESERVED_RAM_MB", 4096)),
		DiskMB:     int64(getenvInt("RESERVED_DISK_MB", 30720)),
	}

	labels := parseLabels(os.Getenv("NODE_LABELS"))
	c := client.New(panelURL, panelToken)
	ctx := context.Background()
	rt := runtime.NewDocker()
	controlServer := agentserver.New(rt, migrationSharedDir, publicAddr)
	httpServer := agentserver.NewHTTPServer(agentAddr, controlServer.Handler())
	go func() {
		log.Printf("agent control api listening on %s", agentAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("agent control api failed: %v", err)
		}
	}()

	registerReq, heartbeatReq, err := metrics.Collect(nodeID, reserve, labels)
	if err != nil {
		log.Fatalf("collect metrics failed: %v", err)
	}
	registerReq.ControlAPIURL = controlURL
	registerReq.PublicAddress = publicAddr
	if err := c.PostJSON(ctx, "/api/nodes/register", registerReq); err != nil {
		log.Fatalf("node registration failed: %v", err)
	}
	log.Printf("node %s registered", nodeID)

	ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		_, heartbeatReq, err = metrics.Collect(nodeID, reserve, labels)
		if err != nil {
			log.Printf("collect metrics failed: %v", err)
			<-ticker.C
			continue
		}
		heartbeatReq.NodeID = nodeID
		if err := c.PostJSON(ctx, "/api/nodes/heartbeat", heartbeatReq); err != nil {
			log.Printf("heartbeat failed: %v", err)
		}
		<-ticker.C
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		parsed, err := strconv.Atoi(v)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func hostNameFallback() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "node-unknown"
	}
	return h
}

func parseLabels(input string) map[string]string {
	if input == "" {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return result
}
