# Bothan

**Bothan** is a self-hosted service that continuously monitors the SSL/TLS
posture of your domains and websites using the [Qualys SSL Labs
API](https://www.ssllabs.com/), tracks grade history, compares scans over time,
and alerts you through multiple notification channels when something changes.

> Status: early development. **Phases 1–2** are implemented — the single binary
> boots, serves its HTTP surface, manages monitored **hosts** (CRUD via API and
> a web UI), applies its database schema, exposes Prometheus metrics, and embeds
> the React web UI. Scanning, scheduling, notifications, comparison, dashboard,
> export/import, and auth arrive in subsequent phases.

## What works today

- Single static binary (`bothan`), CGO-free.
- Configuration via flags, environment, or YAML (precedence: flags > env > YAML).
- SQLite database (pure-Go `modernc.org/sqlite`) with embedded, versioned
  migrations applied at startup.
- **Host management** — add, list, edit, enable/disable, and delete monitored
  hostnames (with per-host public/private, cache, and mismatch options),
  through both the REST API and the web UI.
- HTTP server (chi) with:
  - `GET /healthz` — liveness.
  - `GET /readyz` — readiness (checks the database).
  - `GET /metrics` — Prometheus metrics.
  - `/api/v1/hosts` — host CRUD (JSON error envelope).
  - Embedded React single-page app served at `/` with client-side-route
    fallback and light/dark themes.
- Structured logging via `log/slog` (JSON or text, configurable level).
- Graceful shutdown on `SIGINT` / `SIGTERM`.

## Running

```bash
# build
go build -o bothan ./cmd/bothan

# run against a local database; everything else is configured from the UI
./bothan --db-path ./bothan.db

# optionally pin the bind and provide the encryption key via the environment
BOTHAN_CRYPTO_ENCRYPTION_KEY=... ./bothan --db-path ./bothan.db --port 8080
```

Open the web UI at the bound address, then configure SSL Labs, logging, and the
rest from the **Settings** page.

## Screenshots

### Hosts (light)
![Hosts — light](assets/screenshots/hosts-light.png)

### Hosts (dark)
![Hosts — dark](assets/screenshots/hosts-dark.png)

## Host API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/hosts` | List hosts (ordered by hostname). |
| `POST` | `/api/v1/hosts` | Create a host. Body: `{ "hostname", "publish?", "ignore_mismatch?", "from_cache?", "max_age_hours?", "notes?" }`. Defaults: `enabled=true`, `publish=false` (private). |
| `GET` | `/api/v1/hosts/{id}` | Get a host. |
| `PUT` | `/api/v1/hosts/{id}` | Update a host. |
| `DELETE` | `/api/v1/hosts/{id}` | Delete a host (cascades to its scans). |
| `POST` | `/api/v1/hosts/{id}/enable` | Enable scanning for a host. |
| `POST` | `/api/v1/hosts/{id}/disable` | Disable scanning without deleting. |

## Settings

Bothan has **no YAML configuration file**. Almost all configuration — server
bind, logging, SSL Labs (API version, registered email, poll interval, workers,
scan timeout, default publish), and metrics — is stored in the database and
edited at runtime from the **Settings** page (or the settings API). Changes to
SSL Labs settings and the log level apply immediately; server bind, log format,
and metrics enablement take effect on restart.

### Settings page
![Settings](assets/screenshots/settings-light.png)

### Settings API

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/settings` | Current effective settings. The encryption key is reported only as `encryption_key_set` — never returned. |
| `PUT` | `/api/v1/settings` | Update a partial set of settings (validated, all-or-nothing). |

### Bootstrap (environment/flags only)

Two things cannot live in the database and are provided at startup: the database
path (needed to open the DB) and the encryption key (storing the master key
inside the store it protects would defeat encryption-at-rest). An optional
server-bind override lets a container pin its address regardless of the stored
value.

| Flag | Env | Description |
|---|---|---|
| `--db-path` | `BOTHAN_DATABASE_PATH` | SQLite database path (default `/data/bothan.db`). |
| `--encryption-key` | `BOTHAN_CRYPTO_ENCRYPTION_KEY` | AES-256-GCM key. Prefer the env var; never stored in the DB. Keep it stable and backed up. |
| `--host` | `BOTHAN_SERVER_HOST` | Optional bind host override (wins over the stored value). |
| `--port` | `BOTHAN_SERVER_PORT` | Optional bind port override (wins over the stored value). |
| `--version` | — | Print the version and exit. |

Bootstrap precedence is **flags > environment > default**.

## Frontend development

The web UI lives in `web/` (React + Vite + TypeScript + Tailwind). The build
output is written to `internal/web/dist` and embedded into the binary via
`go:embed`.

```bash
cd web
npm install
npm run dev      # hot-reload dev server, proxies /api to localhost:8080
npm run build    # production build -> internal/web/dist (embedded on next go build)
```

## License

Apache-2.0. See [`LICENSE`](LICENSE).
