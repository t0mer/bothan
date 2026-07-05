# Bothan

**Bothan** is a self-hosted service that continuously monitors the SSL/TLS
posture of your domains and websites using the [Qualys SSL Labs
API](https://www.ssllabs.com/), tracks grade history, compares scans over time,
and alerts you through multiple notification channels when something changes.

> Status: early development. **Phase 1 (skeleton)** is implemented — the single
> binary boots, serves its HTTP surface, applies its database schema, exposes
> Prometheus metrics, and embeds the (placeholder) web UI. Hosts, scanning,
> scheduling, notifications, and the full UI arrive in subsequent phases.

## What works today

- Single static binary (`bothan`), CGO-free.
- Configuration via flags, environment, or YAML (precedence: flags > env > YAML).
- SQLite database (pure-Go `modernc.org/sqlite`) with embedded, versioned
  migrations applied at startup.
- HTTP server (chi) with:
  - `GET /healthz` — liveness.
  - `GET /readyz` — readiness (checks the database).
  - `GET /metrics` — Prometheus metrics.
  - `/api/v1/*` — versioned API surface (JSON error envelope; resources land in
    later phases).
  - Embedded single-page app served at `/` with client-side-route fallback.
- Structured logging via `log/slog` (JSON or text, configurable level).
- Graceful shutdown on `SIGINT` / `SIGTERM`.

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
