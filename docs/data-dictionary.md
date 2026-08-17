# Data Dictionary

The application persists the following tables. All identifiers are
synthetic and carry no PII. Time columns are stored as
`TIMESTAMPTZ` in UTC.

## tenants

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| name | TEXT | Display name. |
| created_at | TIMESTAMPTZ | Default `now()`. |

## users

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| email | TEXT | Synthetic email. |
| role | TEXT | `admin` or `viewer`. |

## products

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| sku | TEXT | Display SKU. |
| name | TEXT | Display name. |
| list_jpy | BIGINT | Integer JPY list price. |
| category | TEXT | Free text. |

Indexed on `(tenant_id, sku)`.

## customers

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| name | TEXT | Display name. |
| segment | TEXT | `standard`, `premium`, `enterprise` or `internal`. |

## contract_prices

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| customer_id | TEXT | Customer reference. |
| sku | TEXT | SKU reference. |
| base_jpy | BIGINT | Contract base price. |
| valid_from | TIMESTAMPTZ | Inclusive start. |
| valid_to | TIMESTAMPTZ | Inclusive end. |

Indexed on `(tenant_id, customer_id, sku, valid_from, valid_to)`.

## eligibility_rules

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| channel | TEXT | Channel code. |
| segment | TEXT | Customer segment. |
| category | TEXT | Product category. |
| valid_from | TIMESTAMPTZ | Inclusive start. |
| valid_to | TIMESTAMPTZ | Inclusive end. |

## promotions

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| name | TEXT | Display name. |
| channel | TEXT | Channel code. |
| product_id | TEXT | SKU reference or NULL. |
| product_scope | TEXT | `sku` or `wildcard`. |
| priority | INT | Higher wins. |
| stacking | BOOLEAN | True if non-exclusive. |
| exclusion | BOOLEAN | True if exclusive. |
| percent_bp | INT | Discount in basis points. |
| valid_from | TIMESTAMPTZ | Inclusive start. |
| valid_to | TIMESTAMPTZ | Inclusive end. |

Indexed on `(tenant_id, channel, valid_from, valid_to)` and
`(tenant_id, product_id)`.

## promotion_conditions

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| promotion_id | TEXT | FK to promotions. |
| kind | TEXT | `quantity`, etc. |
| expr | TEXT | Restricted expression. |

## pricing_requests

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | FK to tenants. |
| customer_id | TEXT | Customer reference. |
| sku | TEXT | SKU reference. |
| quantity | INT | Positive integer. |
| channel | TEXT | Channel code. |
| occurred_at | TIMESTAMPTZ | Request time. |

## pricing_decisions

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| request_id | TEXT | FK to pricing_requests. |
| tenant_id | TEXT | Tenant. |
| customer_id | TEXT | Hashed synthetic id. |
| sku | TEXT | SKU reference. |
| quantity | INT | Echoed. |
| channel | TEXT | Channel code. |
| list_jpy | BIGINT | List price. |
| base_jpy | BIGINT | Effective base (contract overrides list). |
| subtotal_jpy | BIGINT | base × quantity. |
| discount_jpy | BIGINT | Sum of applied discounts. |
| amount_jpy | BIGINT | Final amount. |
| applied_promotion_ids | TEXT | Comma-separated ids. |
| reason | TEXT | Free text reason. |
| config_version | TEXT | Active config version. |
| mode | TEXT | `interactive` or `batch`. |
| created_at | TIMESTAMPTZ | Decision time. |

## batch_jobs

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | Tenant. |
| total_items | INT | Total requests. |
| done_items | INT | Completed requests. |
| status | TEXT | `pending`, `running`, `done`, `failed`. |
| started_at | TIMESTAMPTZ | Job start. |
| completed_at | TIMESTAMPTZ | Job completion. |

## audit_events

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | Tenant. |
| actor_id | TEXT | Hashed synthetic id. |
| action | TEXT | e.g. `price.quote`. |
| entity | TEXT | e.g. `pricing_decision`. |
| entity_id | TEXT | Target id. |
| request_id | TEXT | Optional. |
| notes | TEXT | Synthetic-safe notes. |
| created_at | TIMESTAMPTZ | Event time. |

## config_versions

| Column | Type | Notes |
|--------|------|-------|
| id | TEXT | Primary key. |
| tenant_id | TEXT | Tenant. |
| label | TEXT | Display label. |
| active | BOOLEAN | True for the active version. |
| created_at | TIMESTAMPTZ | Version time. |
