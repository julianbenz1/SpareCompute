package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/panel/ingress"
	panelruntime "github.com/julianbenz1/SpareCompute/internal/panel/runtime"
	"github.com/julianbenz1/SpareCompute/internal/panel/server"
	"github.com/julianbenz1/SpareCompute/internal/panel/store"
)

func main() {
	addr := getenv("PANEL_ADDR", ":8080")
	token := os.Getenv("PANEL_TOKEN")
	dbPath := getenv("PANEL_DB_PATH", "./data/panel.db")
	ingressDynamicConfigPath := os.Getenv("PANEL_INGRESS_DYNAMIC_CONFIG_PATH")
	certResolver := getenv("PANEL_INGRESS_CERT_RESOLVER", "letsencrypt")

	st, err := store.NewSQLite(dbPath)
	if err != nil {
		log.Fatalf("failed to initialize panel store: %v", err)
	}
	srv := &http.Server{
		Addr: addr,
		Handler: server.New(
			st,
			token,
			panelruntime.NewClient(),
			ingress.NewTraefikFileManager(ingressDynamicConfigPath, certResolver),
		).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("sparecompute-panel listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("panel server failed: %v", err)
	}
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
