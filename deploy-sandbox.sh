# Performance Candidate Deployment Script (SOMA Genesis Sandbox)
# This script launches both baseline (port 8080/5173) and candidate (port 8081/5174)
# with identical medium dataset, shared Postgres, and equivalent workload.
# Both remain online until manually stopped.

set -e

echo "=== SOMA Genesis Sandbox — Pricing Performance Validation ==="
echo "Baseline commit: 70b7e68 (Initial commit)"
echo "Candidate commit: 70b7e68 (most recent — identical to baseline)"
echo "Dataset profile: medium (SHA256: ed38e6b32759c01c9618102711bb240521c20e790080a66599000864ed2e46cc)"
echo "Workload: benchmarks/runner.go with 100 rounds on /api/v1/pricing/quote"
echo ""

# Build
echo "Building binaries..."
go build -o bin/api ./cmd/api
go build -o bin/benchmark ./cmd/benchmark
go build -o bin/migrate ./cmd/migrate
go build -o bin/seed ./cmd/seed

# Seed & migrate (shared)
echo "Seeding medium dataset (deterministic)..."
./bin/seed -out testdata/fixtures -profile medium

echo "Running migrations..."
DATABASE_URL="postgres://postgres@localhost:5432/pricing?sslmode=disable" ./bin/migrate -dir migrations

# Start Baseline (ports 8080 API, 5173 frontend)
echo "Starting BASELINE on http://localhost:8080 (API) + http://localhost:5173 (frontend)..."
DATABASE_URL="postgres://postgres@localhost:5432/pricing?sslmode=disable" \
  ./bin/api -port 8080 -metrics-port 9090 > baseline.log 2>&1 &

sleep 3
cd frontend && npm install --silent && npm run build --silent && cd ..
npx serve -s frontend/dist -l 5173 --no-clipboard > frontend-baseline.log 2>&1 &

echo "Starting CANDIDATE on http://localhost:8081 (API) + http://localhost:5174 (frontend)..."
DATABASE_URL="postgres://postgres@localhost:5432/pricing?sslmode=disable" \
  ./bin/api -port 8081 -metrics-port 9091 > candidate.log 2>&1 &

sleep 3
npx serve -s frontend/dist -l 5174 --no-clipboard > frontend-candidate.log 2>&1 &

echo ""
echo "=== Deployments live (both using identical workload/config/dataset) ==="
echo "1. Baseline Application URL: http://localhost:8080"
echo "2. Candidate Application URL: http://localhost:8081"
echo "3. Baseline API Base: http://localhost:8080/api/v1"
echo "   Candidate API Base: http://localhost:8081/api/v1"
echo ""
echo "4. Performance Comparison: Run 'make bench' or ./bin/benchmark -profile medium -rounds 200"
echo "   (P50/P95/P99, throughput, memory, query count captured in benchmark output)"
echo "5. Test Report: Run 'go test ./tests/public/... -json > test-report.json'"
echo "6. Logs: baseline.log, candidate.log, frontend-*.log"
echo "   Profiles: pprof endpoints at http://localhost:8080/debug/pprof and :8081 equivalent"
echo "7. DB Evidence: See pg_stat and EXPLAIN below (run manually or via psql)"
echo "8. Source Commits: Both = 70b7e68 (Initial commit)"
echo "9. Workload: identical medium dataset + benchmark runner (100-200 quote simulations)"
echo "10. User Verification Steps:"
echo "    curl -H 'X-Role: admin' -X POST http://localhost:8080/api/v1/pricing/quote -d '{\"tenant_id\":\"tenant-1\",\"items\":[{\"product_id\":\"prod-1\",\"quantity\":2}]}'"
echo "    Repeat on :8081 and compare response time/latency (use time command or browser devtools)"
echo "11. Remaining Risks/Findings: No measurable difference (identical commits). No persistent dashboard or Grafana in sandbox. Transient processes — will stop on sandbox reset. No claim of improvement (per policy). Residual query compilation cost and cache warming still present."
echo "12. Rollback Procedure: pkill -f 'api -port 8081' && git checkout main && make down (or docker stop if using compose in other env)"
echo ""
echo "Both services are now running. Use the preview_url tool or direct curl for QA."
echo "To stop: pkill -f bin/api && pkill -f serve"
