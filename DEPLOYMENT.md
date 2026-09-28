# Deployment Guide

This app is a single Go binary plus a single SQLite file. There is no
separate application server, cache, or external database to provision.

## Building for production

```bash
CGO_ENABLED=1 go build -o red-numbers .
```

`CGO_ENABLED=1` is required because the `go-sqlite3` driver links SQLite via
CGo. Cross-compiling to a different OS/architecture than the build machine
needs a matching C cross-compiler toolchain; the simplest reliable approach
is to build on (or in a container matching) the target platform.

The stylesheet (`static/style.css`) is embedded into the binary at build time
(`//go:embed static` in `main.go`), so the compiled binary is self-contained —
copy just `red-numbers` to the target host, no `static/` directory required
alongside it. The `migrations/` directory, however, is **not** embedded and
must be deployed alongside the binary (see below).

## Runtime configuration

Set these via environment variables (see `README.md` for the full table):

```bash
export DB_PATH=/var/lib/red-numbers/expenses.db
export PORT=8080
export LOG_LEVEL=info
```

Run it under a process supervisor (systemd, launchd, a container runtime,
etc.) rather than directly in a terminal, so it restarts on crash and starts
on boot.

Example systemd unit:

```ini
[Unit]
Description=Red Numbers expense tracker
After=network.target

[Service]
Environment=DB_PATH=/var/lib/red-numbers/expenses.db
Environment=PORT=8080
Environment=LOG_LEVEL=info
WorkingDirectory=/opt/red-numbers
ExecStart=/opt/red-numbers/red-numbers
Restart=on-failure
User=red-numbers

[Install]
WantedBy=multi-user.target
```

`WorkingDirectory` matters: the migrations directory is resolved as the
relative path `migrations` from the process's current directory, so
`/opt/red-numbers/migrations/*.sql` must exist alongside the binary at
`/opt/red-numbers/red-numbers`.

## Database migration strategy

- Migrations are plain `.sql` files in `migrations/`, named with a numeric
  prefix (`001_...`, `002_...`) that also determines application order.
- On every startup, `database.Initialize` applies any migration not yet
  recorded in the `_migrations` table, inside its own transaction — a
  migration either fully applies or fully rolls back, and a partially-applied
  migration never gets silently skipped on the next boot.
- Adding a new migration: create `NNN_description.sql` with the next number,
  test it locally (`go test ./database/...` includes tests that apply the
  real migration files against a fresh in-memory database), and deploy — it
  runs automatically on the next process start. There is no down-migration
  mechanism; write migrations to be additive/backward-compatible where
  possible (e.g. `ALTER TABLE ... ADD COLUMN` rather than destructive
  rewrites), and test against a copy of production data before rolling out
  anything that rewrites existing rows.
- Startup also seeds default categories (idempotent — `INSERT OR IGNORE`) and
  runs two idempotent backfills: assigning fingerprints to any expense rows
  that predate deduplication, and moving any positive-amount expense into the
  Income category. Both are safe to run on every boot and do nothing once
  the data is already consistent.

## Deployment checklist

- [ ] Binary built with `CGO_ENABLED=1` for the target OS/architecture
- [ ] `migrations/` directory present alongside the binary
- [ ] `DB_PATH` points at a persistent volume/disk (not a container's
      ephemeral filesystem) that survives restarts and redeploys
- [ ] Process supervisor configured to restart on failure
- [ ] `LOG_LEVEL=info` (or `warn` in a very high-traffic setting) so
      `platform.RequestLogging` doesn't flood disk/log-aggregator quotas
- [ ] A reverse proxy (nginx, Caddy, a cloud load balancer) terminates TLS in
      front of the app; the app itself serves plain HTTP. `platform.SecurityHeaders`
      only emits `Strict-Transport-Security` when it sees `r.TLS != nil`, so
      that header only appears once TLS is actually terminated (at the proxy,
      typically) — if you terminate TLS upstream, you may want the proxy to
      add HSTS itself instead, since the Go process never sees the TLS
      connection directly in that topology
- [ ] Backups scheduled (see below) before the first real import
- [ ] `/health` returns 200 from your monitoring/uptime checker

## Scaling considerations

This is a single-SQLite-file, single-process application by design (see
`.kiro/specs/expense-tracking-app/design.md` for the reasoning behind
synchronous, in-process re-classification). Practical implications:

- **Vertical, not horizontal.** SQLite's writer lock means only one process
  should hold `DB_PATH` open for writes at a time. Don't run multiple
  replicas against the same database file. If you need more throughput,
  the first move is a faster disk/more RAM for page cache, not more
  instances.
- **Connection pool.** `main.go` configures `MaxOpenConnections: 10`,
  `MaxIdleConnections: 5`. SQLite handles concurrent readers well; increase
  this only if `GetConnectionPoolStats`-style monitoring (or `/health`
  latency) shows contention, and re-test — more open connections against a
  single SQLite file has diminishing (or negative) returns past a point.
- **Re-classification cost.** Per the design doc, re-classifying ~1,000
  expenses is expected to take under 500ms synchronously. If your expense
  count grows past the tens of thousands, watch the correction endpoint's
  latency (via the request-logging middleware's duration field) — that's the
  first place a synchronous, full-table-scan-per-correction design would show
  strain, and the point at which an async job queue (noted as a future
  option in the design doc) would become worth the added complexity.
- **CSV size.** Uploads are capped at 10MB in the handler; if your bank
  exports get close to that, split the file or raise the limit in
  `domains/upload/router.go`'s `ParseMultipartForm` call (and the matching
  limit implicitly applied by the CSRF middleware's own multipart parse).

## Backup and recovery

The entire application state is one SQLite file at `DB_PATH`.

- **Backup:** copy the file while the app isn't mid-write, or use SQLite's
  online backup approach to avoid corruption from a concurrent write:
  ```bash
  sqlite3 /var/lib/red-numbers/expenses.db ".backup '/backups/expenses-$(date +%F).db'"
  ```
  Run this on a schedule (cron/systemd timer) and store the output off-host.
- **Recovery:** stop the process, replace `DB_PATH` with the backup file,
  restart. Startup will re-verify indexes and re-apply any migrations newer
  than the backup automatically.
- **Verify backups periodically** by actually restoring one into a scratch
  environment and hitting `/health` and the dashboard — an untested backup
  is not a backup.
