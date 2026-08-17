# Architecture

## Module diagram

```
            ┌─────────────────────────────────────────────────────────┐
            │                       Browser (React)                  │
            │  Products · Promotions · Simulator · Batch · Audit    │
            └─────────────────────────────┬───────────────────────────┘
                                          │  /api/v1/*
            ┌─────────────────────────────▼───────────────────────────┐
            │                   HTTP API (chi router)                │
            │   request_id · access_log · bounded body · version     │
            └─────────────────────────────┬───────────────────────────┘
                                          │
            ┌─────────────────────────────▼───────────────────────────┐
            │                    Application                          │
            │  Engine · Compiler · Cache · Aggregator · Selector     │
            └────┬───────────────────┬───────────────────────┬───────┘
                 │                   │                       │
        ┌────────▼────────┐ ┌────────▼────────┐  ┌──────────▼────────┐
        │   Pricing       │ │   Promotion     │  │   Rule engine     │
        │   Engine        │ │   Resolver      │  │   Compiler        │
        └────────┬────────┘ └────────┬────────┘  └──────────┬────────┘
                 │                   │                       │
                 └─────────┬─────────┴───────────┬───────────┘
                           │                     │
                  ┌────────▼─────────┐   ┌───────▼────────┐
                  │ Storage (memory) │   │ Storage (PG)   │
                  │ + Query log      │   │ + Pool metrics │
                  └────────┬─────────┘   └───────┬────────┘
                           │                     │
                           └──────────┬──────────┘
                                      │
                          ┌───────────▼──────────┐
                          │   PostgreSQL 16      │
                          │   (forward migrations)│
                          └──────────────────────┘
```

## Request flow

1. Browser posts to `/api/v1/pricing/quote` with the request
   payload. The chi router applies `requestID` and `accessLog`
   middlewares.
2. The handler validates the request via
   `internal/validation`. Bad input is rejected with `400`.
3. The engine looks up the active `config_version` for the tenant,
   then checks the pricing cache. On a hit the cached decision is
   returned and a `pricing_cache_hits_total` is incremented.
4. The engine loads the product, customer and contract price from
   the storage backend. The candidate promotions are selected
   from the store, conditions are loaded, the resolver orders and
   de-duplicates them, and the rule compiler evaluates the boolean
   conditions.
5. The engine applies each promotion in order, accumulating
   discounts, and rounds the final amount using `internal/money`.
6. The decision is persisted, cached, audited and returned to the
   caller.

## Data ownership

| Layer | Owns |
|-------|------|
| `internal/domain` | Immutable value types. |
| `internal/storage` | Persistence surface (memory + PostgreSQL). |
| `internal/pricing` | Interactive and batch evaluation logic. |
| `internal/promotion` | Candidate selection and conflict resolution. |
| `internal/ruleengine` | Expression compilation and caching. |
| `internal/cache` | Tenant- and configuration-aware result cache. |
| `internal/money` | Integer-JPY arithmetic. |
| `internal/adminsearch` | Parameterised administrative search. |
| `internal/observability` | Structured logs and Prometheus metrics. |
| `internal/security` | Synthetic-safe identifier redaction. |
| `internal/httpapi` | HTTP transport and routing. |
| `cmd/*` | Process entry points. |

## Concurrency model

The pricing engine and the rule compiler are safe for concurrent
use. The cache is guarded by a single RWMutex. The in-memory store
is guarded by a single RWMutex. The PostgreSQL store delegates
concurrency to the underlying connection pool.

## Observability

- Structured JSON access log with `request_id`, `tenant_id`,
  `route`, `method`, `status`, `latency_ms`.
- Per-request query counter, compile counter, cache hit counter and
  candidate count histogram.
- Prometheus metrics exposed at `/metrics`.
- `/version` returns the build commit, build timestamp and dataset
  SHA-256.

## Deployment topology

The `docker-compose.yml` brings up five services:

- `db` — PostgreSQL 16 with a healthcheck and a `pgdata` volume.
- `migrate` — runs the SQL migrations once and exits.
- `api` — the Go HTTP server on port 8080, depending on `migrate`
  having completed.
- `frontend` — Vite dev/preview server on port 5173, depending on
  the API being healthy.
- `seed` — runs the seeder once and writes fixtures to a shared
  volume.
