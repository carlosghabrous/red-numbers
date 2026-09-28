# Red Numbers — Expense Tracking

A small, server-rendered expense tracker for Spanish bank CSV exports. Upload a
statement, get automatic category classification, correct mistakes (with
automatic re-classification of similar expenses), and see spending broken down
with sortable/filterable tables, summary stats, a pie chart, and (soon)
weekly/monthly histograms.

No JavaScript framework, no external database — a single Go binary backed by
SQLite.

## Features

- CSV upload with header validation and duplicate-safe re-imports
- Automatic category classification (keyword-based), with positive amounts
  always classified as **Income**
- Sorting, category filtering, and date-range filtering on the dashboard
- Manual category correction that automatically re-classifies every other
  expense with the same description
- Summary statistics (total, count, average, top category) and a pie chart,
  both filter-aware
- User-defined categories, added from the expense detail page
- CSRF protection, security headers, request logging, and panic recovery on
  every request

## Prerequisites

- Go 1.22 or later
- No separate SQLite install needed — the `go-sqlite3` driver is a CGo
  dependency bundled via `go.mod`, so a C compiler (already present on macOS
  and most Linux distros) is required to build

## Setup and running

```bash
git clone <this-repo>
cd red-numbers

go mod download

# Optional: override defaults (see Configuration below)
export DB_PATH=./expenses.db
export PORT=8080
export LOG_LEVEL=info

go run main.go
```

Then open http://localhost:8080/upload and upload a CSV.

To build a standalone binary (the stylesheet is embedded, so nothing else
needs to ship alongside it):

```bash
go build -o red-numbers .
./red-numbers
```

## CSV format

The parser expects a semicolon-delimited export with these headers (matching
is case-insensitive, and a few metadata rows before the header are tolerated):

| fecha de operación | concepto | fecha valor | importe | saldo |
|---|---|---|---|---|
| 01/01/2024 | Mercadona Compra Supermercado | 01/01/2024 | 45,50 | 1.234,56 |
| 02/01/2024 | Telefonica Pago Factura | 02/01/2024 | 65,00 | 1.169,56 |

- Dates: `DD/MM/YYYY` or `DD-MM-YYYY`
- Amounts: comma (`1.234,56`) or period (`1234.56`) as the decimal separator;
  a positive `importe` is always classified as Income regardless of the
  description
- Rows are de-duplicated by a fingerprint of (date, description, amount,
  balance), so re-uploading the same statement is a no-op

## Configuration

All configuration is via environment variables; every one has a default.

| Variable | Default | Purpose |
|---|---|---|
| `DB_PATH` | `./expenses.db` | Path to the SQLite database file |
| `PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error` |
| `SESSION_SECRET` | `dev-secret-key-change-in-production` | Reserved for future session/auth use; not currently used to sign anything |

## Development

```bash
go build ./...        # compile everything
go vet ./...           # static checks
go test ./...          # unit + integration tests
go test ./... -cover   # with coverage
```

`go test .` (the root package) runs a full end-to-end test that boots the
real HTTP handler chain (routing, CSRF, middleware) against a temporary
SQLite database and drives an upload → dashboard → correction →
re-classification workflow, so it's the fastest way to sanity-check a change
across domains.

### Project layout

```
red-numbers/
├── main.go                  # composition root: config, router wiring, server
├── database/                 # connection setup, migrations, schema verification
├── domains/
│   ├── categories/           # category CRUD + seeding
│   ├── classification/       # keyword classifier + classification log
│   ├── expenses/              # dashboard, detail/correction, statistics, pie chart
│   └── upload/                 # CSV parsing and the upload flow
├── platform/                 # cross-cutting HTTP middleware (CSRF, logging, recovery, headers)
├── static/                    # embedded stylesheet
├── migrations/                # SQL migrations, applied automatically on startup
└── .kiro/specs/                # requirements, design, and task breakdown
```

Each domain owns its own `models.go`, `repository.go` (SQLite access),
`service.go` (business logic), and `router.go` (HTTP handlers + HTML
rendering) — there's no separate template engine; pages are rendered as Go
string templates and share `static/style.css`.

## Further reading

- `.kiro/specs/expense-tracking-app/requirements.md` — what each feature does
- `.kiro/specs/expense-tracking-app/design.md` — architecture and algorithms
- `.kiro/specs/expense-tracking-app/tasks.md` — the implementation plan, slice by slice
- `DEPLOYMENT.md` — running this in production: migrations, backups, scaling
