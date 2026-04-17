# SpareCompute

Eine einfache Open-Source-Plattform, die ungenutzte Restleistung privater Server bündelt.

## MVP-Status

Dieses Repository enthält ein lauffähiges MVP mit:

- **Panel-Server** (Go, HTTP API + einfache Weboberfläche)
- **Node-Agent** (Go, ausgehende Registrierung + Heartbeats)
- **Scheduler** für Erstplatzierung und einfache Migration (controlled replacement)
- **ServiceRoute-Registry** (in-memory)
- **Reserve-/Restleistungslogik** pro Node

> Hinweis: Aktuell ist die Laufzeit- und Registry-Schicht bewusst leichtgewichtig und in-memory.

## Architektur (MVP)

- `cmd/panel`: Startet das zentrale Panel
- `cmd/agent`: Startet einen Node-Agent
- `internal/common`: Gemeinsame Domain-Modelle
- `internal/panel/store`: In-Memory-Datenhaltung
- `internal/panel/scheduler`: Auswahl geeigneter Nodes + Migrationsentscheidung
- `internal/panel/server`: REST-API + eingebettete UI
- `internal/agent/metrics`: Host-Metriken + Berechnung freigegebener Restleistung
- `internal/agent/runtime`: Docker-Adapter (Basis)

## Schnellstart

### Voraussetzungen

- Go 1.22+
- Linux (für aktuelle Agent-Metrikquellen wie `/proc`)

### Panel starten

```bash
cd <repo-root>
go run ./cmd/panel
```

Optional:

- `PANEL_ADDR` (Default `:8080`)
- `PANEL_TOKEN` (Bearer-Token für Agent-Endpoints)

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

## Kern-API (MVP)

- `GET /api/health`
- `POST /api/nodes/register`
- `POST /api/nodes/heartbeat`
- `GET /api/nodes`
- `POST /api/deployments`
- `GET /api/deployments`
- `GET /api/routes`
- `POST /api/reconcile`

## Scheduling- und Migrationslogik (MVP)

- Nur Nodes mit ausreichender **freigegebener** CPU/RAM/Disk werden berücksichtigt.
- Reservewerte werden agentseitig in `shareable_*` eingerechnet.
- Bei `POST /api/reconcile` werden Deployments auf Nodes mit Unterkapazität auf geeignete Ersatznodes verschoben.
- Umschaltung erfolgt über `ServiceRoute.active_instance_id`.

## Build und Tests

```bash
cd <repo-root>
gofmt -w ./cmd ./internal
go test ./...
go build ./...
```

## Bewusste MVP-Grenzen

- In-memory statt persistenter Datenbank
- Keine echte Live-Migration von Containern
- Keine produktionsreife Ingress-/TLS-Automatisierung
- Fokus auf stateless Workloads und controlled replacement
