# API Reference

All endpoints are versioned under `/api/v1`. Request and response
bodies are JSON. The API is unauthenticated in the current build;
production deployments are expected to add a gateway-level identity.

## POST /api/v1/pricing/quote

Runs an interactive pricing decision.

Request:
```json
{
  "tenant_id": "tenant-a",
  "customer_id": "cust-tenant-a-00001",
  "sku": "SKU-tenant-a-00001",
  "quantity": 1,
  "channel": "web"
}
```

Response `200 OK`:
```json
{
  "id": "9f8a...",
  "request_id": "ab12...",
  "tenant_id": "tenant-a",
  "customer_id": "id-3a1b2c4d5e6f",
  "sku": "SKU-tenant-a-00001",
  "quantity": 1,
  "channel": "web",
  "list_jpy": 5000,
  "base_jpy": 5000,
  "subtotal_jpy": 5000,
  "discount_jpy": 250,
  "amount_jpy": 4750,
  "applied_promotion_ids": ["prom-tenant-a-00001"],
  "applied": [
    { "promotion_id": "prom-tenant-a-00001", "reason": "rule matched", "amount_jpy": 250 }
  ],
  "reason": "evaluated",
  "config_version": "cfg-tenant-a",
  "mode": "interactive",
  "created_at": "2026-08-10T08:00:00Z"
}
```

`400` is returned for unknown channels or missing fields. `404` is
returned for unknown product or customer.

## POST /api/v1/pricing/batch

Processes a batch of pricing requests through the same evaluation
path as the interactive endpoint.

Request:
```json
{
  "tenant_id": "tenant-a",
  "items": [
    {
      "tenant_id": "tenant-a",
      "customer_id": "cust-tenant-a-00001",
      "sku": "SKU-tenant-a-00001",
      "quantity": 1,
      "channel": "web"
    }
  ]
}
```

Response `200 OK`:
```json
{
  "decisions": [ { "...": "..." } ],
  "summary": { "count": 1 }
}
```

## GET /api/v1/promotions

Lists promotions for a tenant.

Query parameters:

| Name | Required | Description |
|------|----------|-------------|
| tenant_id | yes | Logical isolation boundary. |
| channel | no | Filter by channel. |
| date | no | RFC3339 timestamp; defaults to now. |

## POST /api/v1/promotions

Creates a new promotion. The server generates a UUID when the
caller omits the id.

## PATCH /api/v1/promotions/{id}

Updates a subset of fields on an existing promotion.

## GET /api/v1/admin/search

Parameterised search over products, customers or promotions.

Query parameters:

| Name | Required | Allowed values |
|------|----------|----------------|
| entity | yes | `products`, `customers`, `promotions` |
| q | no | Free-form text |
| sort | no | Allowlisted column name per entity |
| order | no | `asc` or `desc` |
| limit | no | Positive integer |

## GET /api/v1/admin/audit

Returns audit events for a tenant, newest first.

## POST /api/v1/admin/seed

Populates the in-memory store from the small dataset. Returns the
counts of inserted entities.

## GET /healthz

Liveness probe. Returns `{ "status": "ok", "mode": "...", "ts": "..." }`.

## GET /version

Build identity. Returns `{ "commit": "...", "built_at": "...",
"dataset_id": "..." }`.

## GET /metrics

Prometheus metrics. See `docs/operations.md` for the full list.
