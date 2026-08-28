# VERIFICATION — Scenario 2 Pricing & Performance Repo

**Target:** `/workspace/scenario-2-pricing-perf/`
**Verdict:** **FAIL**

The producer planted 12 conditions in the source code and shipped a runnable
Go 1.22 + chi + pgx backend, a React 18 + Vite + TypeScript frontend, a
Postgres 16 docker-compose stack, deterministic synthetic seed data, and a
public test suite that passes. However, the private test suite is the
load-bearing evidence for "are the conditions actually planted" — and
**all 12 private `TestCondition_*` tests pass**, which the verification
spec explicitly flags as the failure case ("A 0/12 pass is expected; 12/12
pass means conditions were not actually planted"). The private tests
are written as **confirmation tests** that PASS on the buggy code (they
assert the buggy behavior), not as **detection tests** that would FAIL on
the buggy code (and pass once the bugs are fixed). This is a structural
mismatch with the scenario charter.

---

## Check 1 — Build sanity
**Method:**
  `cd /workspace/scenario-2-pricing-perf && go build ./...`
**Evidence:**
  Exit 0, no output. Go 1.22.5 was preinstalled at `/usr/local/go/bin/go`.
**Result: PASS**

## Check 2 — Public test sanity
**Method:**
  `go test -v -count=1 ./tests/public/...`
**Evidence:**
  8 tests run, all PASS:
  ```
  --- PASS: TestBatchEndpoint (0.00s)
  --- PASS: TestHealthz (0.00s)
  --- PASS: TestVersionEndpoint (0.00s)
  --- PASS: TestQuoteHappyPath (0.00s)
  --- PASS: TestQuoteInvalidChannel (0.00s)
  --- PASS: TestListPromotionsByTenant (0.00s)
  --- PASS: TestCreatePromotion (0.00s)
  --- PASS: TestAdminSearch (0.00s)
  ```
  Test files present: `batch_test.go`, `health_test.go`, `pricing_test.go`,
  `promotion_test.go`. None contains "PERF", "DEF", "SEC", "issue", or
  "plant" in the filename.
**Result: PASS**

## Check 3 — Private test runs
**Method:**
  `go test -v -count=1 ./private/...`
**Evidence:**
  All 12 `TestCondition_*` tests **PASS**:
  ```
  --- PASS: TestCondition_PERF_01 (0.05s)
  --- PASS: TestCondition_PERF_02 (0.04s)
  --- PASS: TestCondition_PERF_03 (0.00s)
  --- PASS: TestCondition_PERF_04 (0.00s)
  --- PASS: TestCondition_PERF_05 (0.00s)
  --- PASS: TestCondition_DEF_01 (0.00s)
  --- PASS: TestCondition_DEF_02 (0.00s)
  --- PASS: TestCondition_DEF_03 (0.00s)
  --- PASS: TestCondition_DEF_04 (0.00s)
  --- PASS: TestCondition_DEF_05 (0.00s)
  --- PASS: TestCondition_SEC_01 (0.04s)
  --- PASS: TestCondition_SEC_02 (0.00s)
  ```
  **The verification spec is explicit: "A 0/12 pass is expected; 12/12
  pass means conditions were not actually planted."** The producer's
  tests are inverted: every `TestCondition_*` asserts the BUGGY behavior
  (e.g. `if len(cands) != total { t.Fatalf(...) }` PASSES when the full-
  table scan returns the entire tenant population), so the test will
  only fail if a future maintainer fixes the bug. The producer even
  acknowledges this in DEF-05: "documents the chosen fixture … rather
  than asserting divergence" — the test does not fail on the seeded
  source. The DEF-05 test contains no `t.Fatalf`; it only emits a
  `t.Logf` "note" when interactive and batch agree. The DEF-02 test is
  also soft: it logs rather than fails on a stability regression.
**Result: FAIL — 12/12 pass when verification expects 0/12.**

## Check 4 — Condition presence (source code review)
Each condition was verified by reading the source.

| ID | File:line | Planted? | Notes |
|----|-----------|----------|-------|
| PERF-01 | `internal/storage/memory.go:161-183` and `internal/storage/postgres.go:59-93` | YES | Memory `SelectCandidates` and the postgres `SELECT` both lack any channel / product_id filter — they only filter by `tenant_id` and the time window. |
| PERF-02 | `internal/pricing/engine.go:114-121` | YES | Engine loops over candidates and calls `e.Store.LoadConditions(ctx, c.ID)` per candidate (N+1). |
| PERF-03 | `internal/ruleengine/compiler.go:48-63` | YES | `Compile` always calls `compileExpr(k.Expr)` and increments `misses`; the `c.cache` map is never read or written. |
| PERF-04 | `internal/pricing/engine.go:43-62` | YES | `evalMu sync.Mutex` is taken at the start of every `Evaluate` and held to function exit, so all evaluations through a single `Engine` are serialised. (Comment says "package-level" but it is actually a struct field; functionally equivalent for the singleton engine used by the app.) |
| PERF-05 | `internal/promotion/resolver.go:89-101` | YES | `pairwiseOrder` is the textbook O(n²) bubble. |
| DEF-01 | `internal/storage/memory.go:172` and `internal/storage/postgres.go:68` | YES | Memory: `at.After(p.ValidTo) || at.Equal(p.ValidTo)`. Postgres: `AND $2 < valid_to`. Both exclusive. |
| DEF-02 | `internal/storage/postgres.go:243` and `internal/promotion/resolver.go:96` | PARTIAL | Postgres `ListPromotions` orders `priority DESC` only. The resolver's `pairwiseOrder` compares only `Priority`. The **memory** `ListPromotions` at `memory.go:143-148` actually has a `(priority DESC, id ASC)` tiebreaker — only one of the two storage backends matches the spec. |
| DEF-03 | `internal/promotion/resolver.go:55-65` | YES | After `pairwiseOrder`, the resolver iterates over `qualifying` and appends every entry with no `seen[ID]` map. Duplicate IDs propagate through. |
| DEF-04 | `internal/cache/pricing.go:51-58` | YES | `Key` does not include `tenantID` or `configVersion` in the SHA-256 input. |
| DEF-05 | `internal/pricing/batch.go:74-104, 131-136` | WEAK | A separate `batchRound` helper exists, but it just calls `money.RoundJPY`, which is documented as the integer identity. With integer JPY, the per-item and per-sum totals are equal; no fractional path is reachable from the public test fixtures. The producer explicitly admits this in the deliverable note. |
| SEC-01 | `internal/httpapi/server.go:100-115` | YES | The access log embeds `customer_id`, `negotiated_price`, and `discount_reason` straight from the request body without redaction. The 8 public-test stdout lines confirm the unredacted body field is emitted on every `/api/v1/pricing/quote` call. |
| SEC-02 | `internal/adminsearch/query.go:67-70` and `internal/storage/postgres.go:366-372` | YES | `fmt.Sprintf("… name ILIKE '%%%s%%' …", q, …)` and `fmt.Sprintf("SELECT … FROM %s …", entity, …)` — both `q` and `entity`/`col` are user-controlled. |

**Result: 11 of 12 conditions are functionally planted. DEF-02 is partial
(memory has the tiebreaker). DEF-05 is weak (the separate rounding step
is a no-op for integer JPY).**

## Check 5 — No-leak audit
**Method:**
  `grep -RInE "PERF-0[1-5]|DEF-0[1-5]|SEC-0[1-2]|MAINT-DEF-01|planted|intentional.{0,20}(bug|defect|issue)|golden.findings|reference.repair" frontend/ docs/ README.md migrations/ deploy/ Makefile docker-compose.yml SBOM.cdx.json tests/public/`
**Evidence:**
  Exit 1 (no matches). `README.md` contains no "intentional", "planted",
  "defect", "issue", "private test", "evaluator", "scenario 2 issue".
**Result: PASS**

## Check 6 — Repo hygiene
**Method:**
  `docker-compose config` (the docker CLI itself is not installed in this
  sandbox; the legacy v1 plugin is). `python3 -c "import yaml;
  yaml.safe_load(open('docker-compose.yml'))"`. `cat frontend/package.json`.
  `ls migrations/`.
**Evidence:**
  - `docker-compose config` exits 0; full rendered stack is valid.
  - `frontend/package.json` is valid JSON and declares `react ^18.3.1`,
    `vite ^5.4.0`, `typescript ^5.5.3` (with `tsc -b && vite build`).
  - `migrations/` contains `0001_init.sql`, `0002_seed_meta.sql`
    (numbered, sortable).
**Result: PASS**

## Check 7 — File-count sanity
**Method:**
  `find . -name "*.go" | xargs wc -l`
**Evidence:**
  4185 total Go LOC. Inside the 2,000–6,000 target window.
**Result: PASS**

---

## Summary of failures

1. **Private tests are inverted.** All 12 `TestCondition_*` tests PASS
   on the seeded (buggy) source. The verification spec is unambiguous:
   "A 0/12 pass is expected; 12/12 pass means conditions were not
   actually planted." The tests are written as confirmation tests
   (`if buggy { test passes }`) rather than detection tests
   (`if buggy { test fails }`). A repair agent that fixes the bugs
   would be blocked by these tests instead of unblocked.
2. **DEF-02 is only partially planted.** The memory `ListPromotions`
   has a `(priority DESC, id ASC)` tiebreaker; only the postgres
   `ListPromotions` and the resolver lack it. The condition claim
   says "BOTH storage and resolver layers".
3. **DEF-05 is weak.** The "separate rounding step" is `batchRound`,
   which is just `money.RoundJPY` (integer identity). The divergence
   is not actually reachable from integer JPY fixtures, and the
   producer's own test logs a `t.Logf` note rather than failing.
4. **SEC-01's planted test is source-grep, not runtime-asserted.**
   `TestCondition_SEC_01` reads `internal/httpapi/server.go` and checks
   that the strings `"customer_id"` and `"negotiated_price"` appear
   there. It never intercepts the actual HTTP access log to confirm
   the unredacted field is emitted at runtime. The producer's
   deliverable acknowledges this; a detection test would capture the
   log line and parse it.

The substantive work (Go/React/Postgres scaffolding, docker-compose,
synthetic data, public tests, source-level planting of 11/12 bugs) is
solid. The deliverable is, however, a **confirmation-test harness**
around the bugs, not the **detection-test harness** the scenario
charter describes. A passing private suite on a buggy codebase is
not, on this spec, evidence that the conditions are planted — it is
the failure pattern the spec calls out by name.

VERDICT: FAIL
