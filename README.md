# SpareCompute

Eine einfache Open-Source-Plattform, die ungenutzte Restleistung privater Server bündelt.

## Plattform-Status

Dieses Repository enthält eine lauffähige Basisplattform mit:

- **Panel-Server** (Go, HTTP API + einfache Weboberfläche)
- **Node-Agent** (Go, ausgehende Registrierung + Heartbeats + Runtime-Control-API)
- **Scheduler** für Erstplatzierung und Live-Migration (Checkpoint/Restore)
- **ServiceRoute-Registry** (persistente SQLite-Datenbank)
- **Reserve-/Restleistungslogik** pro Node
- **Ingress-/TLS-Automatisierung** via Traefik Dynamic Config + ACME Resolver

## Architektur (MVP)

- `cmd/panel`: Startet das zentrale Panel
- `cmd/agent`: Startet einen Node-Agent
- `internal/common`: Gemeinsame Domain-Modelle
- `internal/panel/store`: Persistenzschicht (SQLite + Cache)
- `internal/panel/scheduler`: Auswahl geeigneter Nodes + Migrationsentscheidung (Live-Migration)
- `internal/panel/ingress`: Traefik Dynamic-Config Writer
- `internal/panel/server`: REST-API + eingebettete UI
- `internal/agent/metrics`: Host-Metriken + Berechnung freigegebener Restleistung
- `internal/agent/runtime`: Docker-Adapter inkl. Checkpoint/Restore
- `internal/agent/server`: Runtime-Control-API pro Node

## Schnellstart

### Voraussetzungen

- Go 1.24+
- Linux (für aktuelle Agent-Metrikquellen wie `/proc`)

### Panel starten

```bash
cd <repo-root>
go run ./cmd/panel
```

Optional:

- `PANEL_ADDR` (Default `:8080`)
- `PANEL_TOKEN` (Bearer-Token für Agent-Endpoints)
- `PANEL_DB_PATH` (Default `./data/panel.db`)
- `PANEL_INGRESS_DYNAMIC_CONFIG_PATH` (Pfad zur Traefik dynamic config, z. B. `./data/traefik_dynamic.toml`)
- `PANEL_INGRESS_CERT_RESOLVER` (Default `letsencrypt`)

UI: `http://127.0.0.1:8080`

### Agent starten

```bash
cd <repo-root>
PANEL_URL=http://127.0.0.1:8080 \
NODE_ID=node-a \
RESERVED_CPU_PERCENT=20 \
RESERVED_RAM_MB=4096 \
RESERVED_DISK_MB=30720 \
go run ./cmd/agent
```

Optional:

- `PANEL_TOKEN`
- `HEARTBEAT_INTERVAL_SECONDS` (Default `5`)
- `NODE_LABELS` (Format: `region=ch,class=home`)
- `AGENT_ADDR` (Default `:18080`)
- `AGENT_CONTROL_URL` (vom Panel erreichbare Runtime-API-URL)
- `NODE_PUBLIC_ADDRESS` (vom Ingress erreichbare Node-Adresse/IP)
- `MIGRATION_SHARED_DIR` (shared Filesystem für Docker-Checkpoint-Ordner)
- `AGENT_CONTAINER_BIND_HOST` (Default `127.0.0.1`, für externes Ingress z. B. `0.0.0.0`)

## Kern-API (MVP)

- `GET /api/health`
- `POST /api/nodes/register`
- `POST /api/nodes/heartbeat`
- `GET /api/nodes`
- `POST /api/deployments`
- `GET /api/deployments`
- `GET /api/routes`
- `POST /api/reconcile`

## Scheduling- und Migrationslogik

- Nur Nodes mit ausreichender **freigegebener** CPU/RAM/Disk werden berücksichtigt.
- Reservewerte werden agentseitig in `shareable_*` eingerechnet.
- Bei `POST /api/reconcile` werden Deployments auf Nodes mit Unterkapazität auf geeignete Ersatznodes migriert.
- Primärpfad: Docker Checkpoint auf Source + Restore auf Target (`MIGRATION_SHARED_DIR`).
- Fallback: controlled replacement (Start auf Target, danach Stop auf Source).
- Umschaltung erfolgt über `ServiceRoute.active_instance_id`.

## Ingress/TLS-Automatisierung

- Das Panel generiert eine Traefik Dynamic Config-Datei (TOML) für aktive Domains.
- Pro Route wird automatisch ein Router/Service auf die aktive Instanz geschrieben.
- TLS wird pro Route über `certResolver` aktiviert (ACME/Let's Encrypt via Traefik-Static-Config).

## Build und Tests

```bash
cd <repo-root>
gofmt -w ./cmd ./internal
go test ./...
go build ./...
```

## Aktuelle Grenzen

- Live-Migration setzt Docker-Checkpoint/CRIU-Unterstützung und ein gemeinsames Checkpoint-Verzeichnis voraus.
- Ingress/TLS-Automatisierung setzt einen extern gestarteten Traefik mit passender Static-Config voraus.
