package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/julianbenz1/SpareCompute/internal/panel/server"
	"github.com/julianbenz1/SpareCompute/internal/panel/store"
)

func main() {
	addr := getenv("PANEL_ADDR", ":8080")
	token := os.Getenv("PANEL_TOKEN")

	st := store.New()
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(st, token).Handler(),
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

