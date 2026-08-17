#!/usr/bin/env bash
# scripts/smoke.sh — exercises the public API end-to-end against a
# running instance. Exits 0 on success, non-zero on the first failure.
set -euo pipefail
BASE="${BASE:-http://localhost:8080/api/v1}"

echo "== healthz =="
curl -fsS "$BASE/../healthz" > /dev/null

echo "== seed =="
curl -fsS -X POST "$BASE/../admin/seed" > /dev/null || true

echo "== quote =="
curl -fsS -X POST "$BASE/pricing/quote" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "tenant-a",
    "customer_id": "cust-tenant-a-00001",
    "sku": "SKU-tenant-a-00001",
    "quantity": 1,
    "channel": "web"
  }' | tee /tmp/quote.json > /dev/null

echo "== batch =="
curl -fsS -X POST "$BASE/pricing/batch" \
  -H 'Content-Type: application/json' \
  -d '{
    "tenant_id": "tenant-a",
    "items": [
      {"tenant_id":"tenant-a","customer_id":"cust-tenant-a-00001","sku":"SKU-tenant-a-00001","quantity":1,"channel":"web"}
    ]
  }' | tee /tmp/batch.json > /dev/null

echo "== admin search =="
curl -fsS "$BASE/admin/search?entity=products&sort=name&order=asc" | tee /tmp/search.json > /dev/null

echo "smoke test passed"
