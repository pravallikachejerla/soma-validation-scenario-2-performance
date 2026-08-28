# Technical Architecture Specification
**Living Product + Technical Specification**  
**Repository:** pravallikachejerla/soma-validation-scenario-2-performance  
**Last Updated:** 2026-08-28  
**Status:** Living document — update whenever behavior, performance characteristics, or interfaces change.  
**Branch:** genesis/fe2480d4-33d3-4741-aa71-3c4d87a2f52d-proj-repo-pravallikachejerla-soma-validation-scenario-2-performance

This document consolidates the high-level architecture, requirements, data model, and operational characteristics of the pricing, promotion, and channel-eligibility application. It is derived directly from the codebase (domain/types.go, application/app.go, pricing/engine.go, internal modules, docs/*.md, migrations, benchmarks, and deployment artifacts).

## Goal
Provide a high-performance, deterministic pricing engine that supports interactive quote simulation, batch processing, promotion management, administrative search, and audit capabilities while maintaining integer-based money handling, rule-based eligibility, and observable performance characteristics suitable for enterprise validation and performance acceptance testing.

## Users
- **Pricing Analysts / Sales Teams**: Use the Simulator and Products pages to generate real-time quotes.
- **Promotion Managers**: Create, edit, and manage promotions via the Promotions page.
- **Administrators**: Perform batch pricing, audit reviews, and administrative searches.
- **Operations / SRE**: Monitor via metrics, logs, health checks, and performance acceptance reports.
- **Reviewers / Testers**: Execute public and private test suites against deterministic fixtures.

## Features
- Interactive pricing (`/api/v1/pricing/quote`) with promotion resolution and rule evaluation.
- Batch pricing (`/api/v1/pricing/batch`).
- Promotion CRUD with conditions and eligibility rules.
- Admin search across products, customers, and promotions.
- Audit log viewer.
- Deterministic synthetic dataset generation (small/medium/large profiles).
- In-memory and PostgreSQL storage backends.
- React + TypeScript frontend with pages for Products, Promotions, Simulator, Batch, Explanation, Audit, and AdminSearch.
- Prometheus metrics and structured JSON logging.
- Performance benchmarking suite.

## Workflows
1. **Interactive Quote**: User submits PricingRequest → validation → cache lookup → load product/customer/contract → select candidate promotions → resolve conflicts → compile & evaluate rules → apply discounts (money package) → persist decision → cache → audit → return.
2. **Batch Pricing**: Submit batch job → worker processes items in parallel → aggregates results → updates job status.
3. **Promotion Management**: Admin creates/updates promotion + conditions → stored with validity windows → used by resolver during pricing.
4. **Admin Search**: Parameterized queries with sorting, filtering, and pagination.
5. **Seeding & Migration**: `cmd/seed` generates fixtures → `cmd/migrate` applies SQL → services start against configured store.

## Data Model
See `internal/domain/types.go` and `docs/data-dictionary.md` for full definitions. Core entities:

- **Tenant**, **User** (with roles: admin/viewer)
- **Product** (SKU, list_jpy, category)
- **Customer** (segment: standard/premium/enterprise/internal)
- **ContractPrice**, **EligibilityRule**, **Promotion** (priority, stacking, exclusion, percent_bp, validity windows)
- **PromotionCondition** (expr for rule engine)
- **PricingRequest**, **PricingDecision** (with applied promotions, amounts in integer JPY)
- **BatchJob**, **AuditEvent**, **ConfigVersion**

All monetary values use `internal/money` (integer JPY only — no floats).

## Business Rules
- Promotions are selected by channel, segment, category, and product scope (sku or wildcard).
- Higher priority wins; stacking vs. exclusive logic applied by `internal/promotion/resolver.go` and `selector.go`.
- Conditions compiled by `internal/ruleengine/compiler.go` into evaluable expressions (limited grammar: comparisons, &&, ||).
- Final amount rounded using `internal/money`.
- Decisions are versioned by active `config_version`.
- Synthetic identifiers only — security redaction via `internal/security/redact.go` and SHA hashing.

## Constraints
- Money is strictly integer JPY (no decimals, no other currencies).
- Rule language is intentionally restricted (not a full expression engine).
- Cache is process-local (no distributed cache).
- No real authentication beyond synthetic `X-Role` header.
- Synthetic dataset only — no production PII.

## Non-Functional Requirements
- **Performance**: Target <1ms P50 interactive quote on medium dataset (see `docs/performance-acceptance.md` for 2.5x improvement via larger cache and pre-wiring).
- **Observability**: Structured logs, Prometheus metrics (query_count, compile_count, cache_hits, evaluate_duration, candidate_count), /healthz, /version, /metrics.
- **Determinism**: All fixtures generated from fixed seed 42 with verifiable SHA-256.
- **Concurrency**: Pricing engine and compiler are concurrent-safe; cache and in-memory store protected by RWMutex.
- **Deployment**: Docker Compose with separate db, migrate, api, frontend, seed services.

## Edge Cases
- Zero-quantity requests, expired promotions, overlapping validity windows, conflicting stacking/exclusion rules, cache eviction under load, missing contract prices, rule compilation failures, large batch jobs (memory pressure), admin search with complex filters.
- Handled in `internal/domain/errors.go`, validation package, and private test suite (`private/condition_test.go`, `private/clean_baseline_test.go`).

## Acceptance Criteria
- All public tests (`go test ./tests/public/...`) pass against small/medium fixtures.
- Performance acceptance thresholds met (latency, throughput, memory — documented in `docs/performance-acceptance.md`).
- DOCX version of this architecture document exists and is up-to-date.
- Frontend builds and connects to API.
- Docker Compose stack starts cleanly with healthchecks.
- No secrets in code; SBOM present; security redaction functional.

## Non-Goals
- Real payment/shipping/tax integration.
- Multi-region or distributed caching.
- Production IAM or quota enforcement.
- General-purpose rule language.
- Float-based monetary arithmetic.
- Production-scale concurrent load testing inside this sandbox (measured in CI).

## Architecture Rules

### Current Architecture and Boundaries
Clean hexagonal-style layering with strict internal boundaries. Frontend talks only to HTTP API. Business logic lives in internal packages; no business logic in `cmd/` or `httpapi`.

### Modules
- **cmd/**: Entry points (api, benchmark, migrate, seed, worker).
- **internal/domain**: Core immutable types and errors.
- **internal/pricing**: Engine, aggregator, batch logic.
- **internal/promotion**: Resolver, selector, conflict handling.
- **internal/ruleengine**: Compiler and cached evaluation.
- **internal/cache**: Tenant-aware pricing result cache.
- **internal/storage**: MemoryStore and PGStore implementations.
- **internal/application**: Wiring of Engine, Compiler, Cache, Metrics.
- **internal/httpapi**: Chi router, middleware (request_id, logging, security).
- **internal/adminsearch**, **internal/observability**, **internal/money**, **internal/security**, **internal/validation**.
- **frontend/**: React + TS + Vite UI.
- **migrations/**: Forward-only SQL.
- **tests/public/**, **private/**: Test suites.
- **benchmarks/**: Performance runner.

### Layers
1. **Presentation** — React frontend + HTTP handlers.
2. **Application** — Orchestration, validation, caching.
3. **Domain** — Business entities, pricing logic, rule evaluation.
4. **Infrastructure** — Storage (memory/PG), observability, security primitives.

### Allowed Dependencies
- Domain types may be used by all layers.
- Application may depend on pricing, promotion, ruleengine, cache, storage, observability.
- HTTP layer depends only on application and httpapi middleware.
- Storage implementations may depend on database drivers and domain.

### Forbidden Dependencies
- No direct database access from pricing engine or frontend.
- No circular dependencies between pricing/promotion/ruleengine.
- Frontend must not contain business logic.
- No external service calls (no real payment gateways, etc.).
- Tests must not depend on private implementation details outside their designated suite.

### Data Flow
Browser → HTTP (chi) → Validation → Cache hit? → Application → Pricing Engine → Promotion Resolver → Rule Compiler → Storage (Mem/PG) → Money rounding → Decision persistence → Cache write → Audit → Response. Batch flows through worker. All flows emit metrics and structured logs.

### Public Interfaces
- REST API under `/api/v1` (see `docs/api.md` for full spec).
- Prometheus `/metrics`.
- Health (`/healthz`), version (`/version`).
- Frontend at `:5173`.

### Integration Points
- PostgreSQL 16 (via pgx).
- Docker Compose for local stack.
- Prometheus for metrics.
- Deterministic fixtures via `cmd/seed` and `testdata/fixtures/`.

## Implementation Notes
- Performance improvements (larger cache, compiler pre-wiring) documented in `docs/performance-acceptance.md`.
- All monetary math in `internal/money/money.go`.
- Security via redaction and SHA in `internal/security/`.
- Observability centralized in `internal/observability/`.

This document serves as the single source of truth for architecture and requirements. Update it whenever any module, interface, performance characteristic, or behavior changes. The accompanying `technical-architecture.docx` is the exported snapshot.

**End of Document**
