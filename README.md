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

## Running

```bash
# build
go build -o bothan ./cmd/bothan

# run against a local database on port 8080
./bothan --db-path ./bothan.db --port 8080 --log-format text
```

Copy `config.yaml.example` to `config.yaml` and pass `--config config.yaml` to
use a file instead of flags/env.

## CLI flags

| Flag | Config key | Description |
|---|---|---|
| `--config` | — | Path to a YAML config file. |
| `--host` | `server.host` | HTTP listen host. |
| `--port` | `server.port` | HTTP listen port. |
| `--base-path` | `server.base_path` | Base path for reverse-proxy sub-paths. |
| `--db-path` | `database.path` | SQLite database path. |
| `--log-level` | `log.level` | `debug` \| `info` \| `warn` \| `error`. |
| `--log-format` | `log.format` | `json` \| `text`. |
| `--version` | — | Print the version and exit. |

## Environment variables

Every config key is settable via an environment variable using the `BOTHAN_`
prefix with dots replaced by underscores:

| Variable | Config key |
|---|---|
| `BOTHAN_SERVER_PORT` | `server.port` |
| `BOTHAN_DATABASE_PATH` | `database.path` |
| `BOTHAN_SSLLABS_API_VERSION` | `ssllabs.api_version` |
| `BOTHAN_SSLLABS_EMAIL` | `ssllabs.email` |
| `BOTHAN_CRYPTO_ENCRYPTION_KEY` | `crypto.encryption_key` |
| `BOTHAN_LOG_LEVEL` | `log.level` |

See `config.yaml.example` for the full set of options.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
