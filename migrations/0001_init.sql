-- 0001_init.sql — initial schema for the pricing application.
-- All identifiers are synthetic. The schema is conservative: every
-- table carries a primary key, a tenant_id and timestamps.

CREATE TABLE IF NOT EXISTS tenants (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    email      TEXT NOT NULL,
    role       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    sku        TEXT NOT NULL,
    name       TEXT NOT NULL,
    list_jpy   BIGINT NOT NULL,
    category   TEXT
);
CREATE INDEX IF NOT EXISTS idx_products_tenant_sku ON products (tenant_id, sku);

CREATE TABLE IF NOT EXISTS customers (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    segment    TEXT
);
CREATE INDEX IF NOT EXISTS idx_customers_tenant ON customers (tenant_id);

CREATE TABLE IF NOT EXISTS contract_prices (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id),
    customer_id  TEXT NOT NULL,
    sku          TEXT NOT NULL,
    base_jpy     BIGINT NOT NULL,
    valid_from   TIMESTAMPTZ NOT NULL,
    valid_to     TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_contract_prices_lookup
    ON contract_prices (tenant_id, customer_id, sku, valid_from, valid_to);

CREATE TABLE IF NOT EXISTS eligibility_rules (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    channel     TEXT NOT NULL,
    segment     TEXT,
    category    TEXT,
    valid_from  TIMESTAMPTZ NOT NULL,
    valid_to    TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_eligibility_rules_lookup
    ON eligibility_rules (tenant_id, channel, valid_from, valid_to);

CREATE TABLE IF NOT EXISTS promotions (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id),
    name          TEXT NOT NULL,
    channel       TEXT NOT NULL,
    product_id    TEXT,
    product_scope TEXT NOT NULL,
    priority      INT NOT NULL,
    stacking      BOOLEAN NOT NULL,
    exclusion     BOOLEAN NOT NULL,
    percent_bp    INT NOT NULL,
    valid_from    TIMESTAMPTZ NOT NULL,
    valid_to      TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_promotions_lookup
    ON promotions (tenant_id, channel, valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_promotions_product
    ON promotions (tenant_id, product_id);

CREATE TABLE IF NOT EXISTS promotion_conditions (
    id            TEXT PRIMARY KEY,
    promotion_id  TEXT NOT NULL REFERENCES promotions(id),
    kind          TEXT NOT NULL,
    expr          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_promotion_conditions ON promotion_conditions (promotion_id);

CREATE TABLE IF NOT EXISTS pricing_requests (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    customer_id TEXT NOT NULL,
    sku         TEXT NOT NULL,
    quantity    INT NOT NULL,
    channel     TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pricing_requests_tenant ON pricing_requests (tenant_id, occurred_at);

CREATE TABLE IF NOT EXISTS pricing_decisions (
    id                     TEXT PRIMARY KEY,
    request_id             TEXT NOT NULL,
    tenant_id              TEXT NOT NULL,
    customer_id            TEXT NOT NULL,
    sku                    TEXT NOT NULL,
    quantity               INT NOT NULL,
    channel                TEXT NOT NULL,
    list_jpy               BIGINT NOT NULL,
    base_jpy               BIGINT NOT NULL,
    subtotal_jpy           BIGINT NOT NULL,
    discount_jpy           BIGINT NOT NULL,
    amount_jpy             BIGINT NOT NULL,
    applied_promotion_ids  TEXT NOT NULL,
    reason                 TEXT NOT NULL,
    config_version         TEXT NOT NULL,
    mode                   TEXT NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_pricing_decisions_tenant ON pricing_decisions (tenant_id, created_at);

CREATE TABLE IF NOT EXISTS batch_jobs (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL,
    total_items   INT NOT NULL,
    done_items    INT NOT NULL,
    status        TEXT NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL,
    completed_at  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS audit_events (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    actor_id    TEXT NOT NULL,
    action      TEXT NOT NULL,
    entity      TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    request_id  TEXT,
    notes       TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_events_tenant ON audit_events (tenant_id, created_at);

CREATE TABLE IF NOT EXISTS config_versions (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL,
    label       TEXT NOT NULL,
    active      BOOLEAN NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_config_versions_tenant ON config_versions (tenant_id, active);
