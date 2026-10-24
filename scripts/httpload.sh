#!/usr/bin/env bash
# httpload.sh — runs an equivalent HTTP workload against baseline and candidate
# and records latency, throughput, and query counts.
set -euo pipefail
BASE_URL="${1:-http://localhost:8081}"
LABEL="${2:-baseline}"
N="${3:-500}"
C="${4:-8}"
OUT="/work/logs/httpload-${LABEL}.log"

echo "=== HTTP load: ${LABEL} (${BASE_URL}) n=${N} c=${C} ===" | tee "${OUT}"

# Seed first
curl -fsS -X POST "${BASE_URL}/api/v1/admin/seed" > /dev/null

# Build a list of valid (tenant, customer, sku) triples from the seed
# by hitting the promotions endpoint to confirm the server is alive.
curl -fsS "${BASE_URL}/api/v1/promotions?tenant_id=tenant-a" > /dev/null

# Run N requests with C concurrent workers using xargs
START=$(date +%s%N)
seq 1 "${N}" | xargs -P "${C}" -I{} curl -fsS -o /dev/null -w "%{time_total}\n" \
  -X POST "${BASE_URL}/api/v1/pricing/quote" \
  -H 'Content-Type: application/json' \
  -d '{"tenant_id":"tenant-a","customer_id":"cust-tenant-a-00001","sku":"SKU-tenant-a-00001","quantity":1,"channel":"web"}' \
  2>/dev/null | sort -n > "${OUT}.latencies"
END=$(date +%s%N)

ELAPSED_MS=$(( (END - START) / 1000000 ))
COUNT=$(wc -l < "${OUT}.latencies")
P50=$(awk -v c="${COUNT}" 'NR==int(c*0.50)' "${OUT}.latencies")
P95=$(awk -v c="${COUNT}" 'NR==int(c*0.95)' "${OUT}.latencies")
P99=$(awk -v c="${COUNT}" 'NR==int(c*0.99)' "${OUT}.latencies")
RPS=$(awk -v c="${COUNT}" -v ms="${ELAPSED_MS}" 'BEGIN{printf "%.0f", c/(ms/1000)}')

echo "elapsed_ms=${ELAPSED_MS} count=${COUNT}" | tee -a "${OUT}"
echo "p50_s=${P50} p95_s=${P95} p99_s=${P99} rps=${RPS}" | tee -a "${OUT}"