# Operations

## Environment variables

| Name | Default | Description |
|------|---------|-------------|
| `APP_MODE` | `memory` | `memory` or `postgres`. |
| `APP_ADDR` | `:8080` | HTTP listen address. |
| `DATABASE_URL` | `postgres://pricing:pricing@localhost:5432/pricing?sslmode=disable` | PostgreSQL DSN. |
| `BUILD_COMMIT` | `dev` | Commit hash exposed at `/version`. |
| `BUILD_BUILT_AT` | (now) | Build timestamp exposed at `/version`. |
| `BUILD_DATASET_ID` | empty | Dataset SHA-256 exposed at `/version`. |

## Healthchecks

The API exposes a `GET /healthz` endpoint that returns `200 OK`
when the process is alive and the storage layer is reachable. The
docker compose file wires this endpoint into the container
healthcheck.

The PostgreSQL container uses the official `pg_isready` healthcheck.

## Metrics

The following Prometheus metrics are exposed at `/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `pricing_query_count` | Counter | Database queries during pricing. |
| `pricing_compile_count` | Counter | Rule expression compilations. |
| `pricing_evaluate_duration_seconds` | Histogram | Pricing evaluation duration. |
| `pricing_cache_hits_total` | Counter | Pricing cache hits. |
| `pricing_candidate_count` | Histogram | Promotion candidates per request. |

In-process counters are also accessible through
`observability.Metrics` for tests.

## Log fields

The structured access log carries the following fields per
request:

| Field | Description |
|-------|-------------|
| `ts` | RFC3339Nano timestamp. |
| `level` | `info`, `warn`, `error`. |
| `msg` | `http_request` for the access log. |
| `request_id` | UUID attached by the middleware. |
| `route` | Request path. |
| `method` | HTTP method. |
| `status` | HTTP status code. |
| `latency_ms` | Wall-clock duration in milliseconds. |

Logs never carry raw customer identifiers or negotiated prices.
Where a synthetic identifier is required, the request is hashed
and only the first 12 hex characters are emitted.

## Rollback

`make down` stops the docker compose stack. The `pgdata` volume is
removed so the next start begins from a fresh schema.
