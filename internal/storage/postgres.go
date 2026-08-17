// Package storage provides the PostgreSQL-backed implementation of the
// pricing repository. It mirrors the MemoryStore surface so the
// application can switch backends via configuration.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// PGStore is the database-backed implementation.
type PGStore struct {
	db      *sql.DB
	queries []string
}

// NewPGStore opens a *sql.DB pool for the given DSN.
func NewPGStore(db *sql.DB) *PGStore {
	return &PGStore{db: db}
}

// DB returns the underlying connection pool.
func (p *PGStore) DB() *sql.DB { return p.db }

// RecordQuery keeps a query signature log for the same observability
// hooks used by MemoryStore. The actual SQL is sent through the driver.
func (p *PGStore) RecordQuery(q string) {
	p.queries = append(p.queries, q)
}

// QueryLog returns recorded query signatures.
func (p *PGStore) QueryLog() []string {
	out := make([]string, len(p.queries))
	copy(out, p.queries)
	return out
}

// SelectCandidates returns the active promotions that match the
// request predicates. The clean path uses an indexed query:
//
//   SELECT id, ... FROM promotions
//    WHERE tenant_id = $1
//      AND channel   = $2
//      AND (product_id = $3 OR product_id = '' OR product_scope = 'wildcard')
//      AND valid_from <= $4 AND valid_to >= $4
//    ORDER BY priority DESC, id ASC
//
// Candidate selection is implemented as a tenant scan.
// predicate except tenant_id and the "active" flag, then sorts only
// by priority. The result set is the same cardinality the clean path
// would return for an empty result, but the candidate count is now
// equal to the total active population for the tenant.
func (p *PGStore) SelectCandidates(ctx context.Context, tenantID, channel, productID string, at time.Time) ([]domain.Promotion, error) {
	p.RecordQuery("SelectCandidates")
	// SEEDED PATH: scan every active promotion for the tenant.
	const q = `
SELECT id, tenant_id, name, channel, COALESCE(product_id, ''), product_scope,
       priority, stacking, exclusion, percent_bp, valid_from, valid_to
  FROM promotions
 WHERE tenant_id = $1
   AND valid_from <= $2
   AND $2 < valid_to
 ORDER BY priority DESC`
	rows, err := p.db.QueryContext(ctx, q, tenantID, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Promotion
	for rows.Next() {
		var (
			p           domain.Promotion
			validFrom   time.Time
			validTo     time.Time
			productIDNS sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Channel, &productIDNS, &p.ProductScope,
			&p.Priority, &p.Stacking, &p.Exclusion, &p.PercentBP, &validFrom, &validTo); err != nil {
			return nil, err
		}
		p.ProductID = productIDNS.String
		p.ValidFrom = validFrom
		p.ValidTo = validTo
		out = append(out, p)
	}
	return out, rows.Err()
}

// LoadConditions returns the conditions for a single promotion.
//
// Conditions are loaded per-promotion.
// query. Seeded path issues one query per promotion. The resolver
// loop in resolver.go calls this once per candidate.
func (p *PGStore) LoadConditions(ctx context.Context, promotionID string) ([]domain.PromotionCondition, error) {
	p.RecordQuery("LoadConditions:" + promotionID)
	const q = `SELECT id, promotion_id, kind, expr FROM promotion_conditions WHERE promotion_id = $1`
	rows, err := p.db.QueryContext(ctx, q, promotionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PromotionCondition
	for rows.Next() {
		var c domain.PromotionCondition
		if err := rows.Scan(&c.ID, &c.PromotionID, &c.Kind, &c.Expr); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// LoadAllConditions returns conditions for many promotions in a single
// query. This is the clean path; the seeded resolver falls back to
// LoadConditions to expose the N+1.
func (p *PGStore) LoadAllConditions(ctx context.Context, promotionIDs []string) (map[string][]domain.PromotionCondition, error) {
	if len(promotionIDs) == 0 {
		return map[string][]domain.PromotionCondition{}, nil
	}
	p.RecordQuery("LoadAllConditions")
	placeholders := make([]string, len(promotionIDs))
	args := make([]any, len(promotionIDs))
	for i, id := range promotionIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	q := "SELECT id, promotion_id, kind, expr FROM promotion_conditions WHERE promotion_id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]domain.PromotionCondition{}
	for rows.Next() {
		var c domain.PromotionCondition
		if err := rows.Scan(&c.ID, &c.PromotionID, &c.Kind, &c.Expr); err != nil {
			return nil, err
		}
		out[c.PromotionID] = append(out[c.PromotionID], c)
	}
	return out, rows.Err()
}

// LookupProduct returns a product by SKU within a tenant.
func (p *PGStore) LookupProduct(ctx context.Context, tenantID, sku string) (domain.Product, error) {
	p.RecordQuery("LookupProduct")
	const q = `SELECT id, tenant_id, sku, name, list_jpy, COALESCE(category, '') FROM products WHERE tenant_id = $1 AND sku = $2`
	var prod domain.Product
	err := p.db.QueryRowContext(ctx, q, tenantID, sku).Scan(&prod.ID, &prod.TenantID, &prod.SKU, &prod.Name, &prod.ListJPY, &prod.Category)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Product{}, domain.ErrNotFound
	}
	return prod, err
}

// LookupCustomer returns a customer.
func (p *PGStore) LookupCustomer(ctx context.Context, tenantID, customerID string) (domain.Customer, error) {
	p.RecordQuery("LookupCustomer")
	const q = `SELECT id, tenant_id, name, COALESCE(segment, '') FROM customers WHERE tenant_id = $1 AND id = $2`
	var c domain.Customer
	err := p.db.QueryRowContext(ctx, q, tenantID, customerID).Scan(&c.ID, &c.TenantID, &c.Name, &c.Segment)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Customer{}, domain.ErrNotFound
	}
	return c, err
}

// LookupContractPrice returns the contract price for a customer/sku if any.
func (p *PGStore) LookupContractPrice(ctx context.Context, tenantID, customerID, sku string, at time.Time) (domain.ContractPrice, bool) {
	p.RecordQuery("LookupContractPrice")
	const q = `
SELECT id, tenant_id, customer_id, sku, base_jpy, valid_from, valid_to
  FROM contract_prices
 WHERE tenant_id = $1 AND customer_id = $2 AND sku = $3
   AND valid_from <= $4 AND valid_to >= $4
 ORDER BY valid_from DESC LIMIT 1`
	var (
		c        domain.ContractPrice
		validFrm time.Time
		validTo  time.Time
	)
	err := p.db.QueryRowContext(ctx, q, tenantID, customerID, sku, at).Scan(&c.ID, &c.TenantID, &c.CustomerID, &c.SKU, &c.BaseJPY, &validFrm, &validTo)
	if err != nil {
		return domain.ContractPrice{}, false
	}
	c.ValidFrom = validFrm
	c.ValidTo = validTo
	return c, true
}

// GetActiveConfig returns the active config version for a tenant.
func (p *PGStore) GetActiveConfig(ctx context.Context, tenantID string) (domain.ConfigVersion, error) {
	p.RecordQuery("GetActiveConfig")
	const q = `SELECT id, tenant_id, label, active, created_at FROM config_versions WHERE tenant_id = $1 AND active = TRUE ORDER BY created_at DESC LIMIT 1`
	var (
		v         domain.ConfigVersion
		createdAt time.Time
	)
	err := p.db.QueryRowContext(ctx, q, tenantID).Scan(&v.ID, &v.TenantID, &v.Label, &v.Active, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ConfigVersion{}, domain.ErrNoConfigVersion
	}
	v.CreatedAt = createdAt
	return v, err
}

// GetPromotion loads a single promotion.
func (p *PGStore) GetPromotion(id string) (domain.Promotion, bool) {
	const q = `
SELECT id, tenant_id, name, channel, COALESCE(product_id, ''), product_scope,
       priority, stacking, exclusion, percent_bp, valid_from, valid_to
  FROM promotions WHERE id = $1`
	var (
		prom        domain.Promotion
		validFrom   time.Time
		validTo     time.Time
		productIDNS sql.NullString
	)
	err := p.db.QueryRow(q, id).Scan(&prom.ID, &prom.TenantID, &prom.Name, &prom.Channel, &productIDNS, &prom.ProductScope,
		&prom.Priority, &prom.Stacking, &prom.Exclusion, &prom.PercentBP, &validFrom, &validTo)
	if err != nil {
		return domain.Promotion{}, false
	}
	prom.ProductID = productIDNS.String
	prom.ValidFrom = validFrom
	prom.ValidTo = validTo
	return prom, true
}

// ListPromotions returns all promotions for a tenant.
//
// No secondary tie-breaker; ordering is by priority only.
func (p *PGStore) ListPromotions(tenantID string) []domain.Promotion {
	const q = `
SELECT id, tenant_id, name, channel, COALESCE(product_id, ''), product_scope,
       priority, stacking, exclusion, percent_bp, valid_from, valid_to
  FROM promotions WHERE tenant_id = $1 ORDER BY priority DESC`
	rows, err := p.db.Query(q, tenantID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []domain.Promotion
	for rows.Next() {
		var (
			p           domain.Promotion
			validFrom   time.Time
			validTo     time.Time
			productIDNS sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Name, &p.Channel, &productIDNS, &p.ProductScope,
			&p.Priority, &p.Stacking, &p.Exclusion, &p.PercentBP, &validFrom, &validTo); err != nil {
			return nil
		}
		p.ProductID = productIDNS.String
		p.ValidFrom = validFrom
		p.ValidTo = validTo
		out = append(out, p)
	}
	return out
}

// PutPromotion inserts or replaces a promotion and its conditions.
func (p *PGStore) PutPromotion(ctx context.Context, prom domain.Promotion, conds []domain.PromotionCondition) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	const up = `
INSERT INTO promotions (id, tenant_id, name, channel, product_id, product_scope,
                         priority, stacking, exclusion, percent_bp, valid_from, valid_to)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, channel=EXCLUDED.channel,
  product_id=EXCLUDED.product_id, product_scope=EXCLUDED.product_scope,
  priority=EXCLUDED.priority, stacking=EXCLUDED.stacking, exclusion=EXCLUDED.exclusion,
  percent_bp=EXCLUDED.percent_bp, valid_from=EXCLUDED.valid_from, valid_to=EXCLUDED.valid_to`
	var pid any
	if prom.ProductID == "" {
		pid = nil
	} else {
		pid = prom.ProductID
	}
	if _, err := tx.ExecContext(ctx, up, prom.ID, prom.TenantID, prom.Name, prom.Channel, pid, prom.ProductScope,
		prom.Priority, prom.Stacking, prom.Exclusion, prom.PercentBP, prom.ValidFrom, prom.ValidTo); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM promotion_conditions WHERE promotion_id = $1`, prom.ID); err != nil {
		return err
	}
	for _, c := range conds {
		if _, err := tx.ExecContext(ctx, `INSERT INTO promotion_conditions (id, promotion_id, kind, expr) VALUES ($1,$2,$3,$4)`,
			c.ID, c.PromotionID, c.Kind, c.Expr); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SaveRequest persists the request.
func (p *PGStore) SaveRequest(ctx context.Context, r domain.PricingRequest) error {
	const q = `INSERT INTO pricing_requests (id, tenant_id, customer_id, sku, quantity, channel, occurred_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := p.db.ExecContext(ctx, q, r.ID, r.TenantID, r.CustomerID, r.SKU, r.Quantity, r.Channel, r.OccurredAt)
	return err
}

// SaveDecision persists the decision.
func (p *PGStore) SaveDecision(ctx context.Context, d domain.PricingDecision) error {
	const q = `INSERT INTO pricing_decisions (id, request_id, tenant_id, customer_id, sku, quantity, channel,
		             list_jpy, base_jpy, subtotal_jpy, discount_jpy, amount_jpy,
		             applied_promotion_ids, reason, config_version, mode, created_at)
	         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`
	ids := strings.Join(d.AppliedPromotionIDs, ",")
	_, err := p.db.ExecContext(ctx, q, d.ID, d.RequestID, d.TenantID, d.CustomerID, d.SKU, d.Quantity, d.Channel,
		d.ListJPY, d.BaseJPY, d.SubtotalJPY, d.DiscountJPY, d.AmountJPY, ids, d.Reason, d.ConfigVersion, d.Mode, d.CreatedAt)
	return err
}

// AppendAudit records an audit event.
func (p *PGStore) AppendAudit(ctx context.Context, ev domain.AuditEvent) error {
	const q = `INSERT INTO audit_events (id, tenant_id, actor_id, action, entity, entity_id, request_id, notes, created_at)
	           VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	_, err := p.db.ExecContext(ctx, q, ev.ID, ev.TenantID, ev.ActorID, ev.Action, ev.Entity, ev.EntityID, nullString(ev.RequestID), ev.Notes, ev.CreatedAt)
	return err
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Search runs the parameterised administrative search.
//
// The admin search builds SQL with string interpolation.
// the user-supplied `sort` and `q` values concatenated directly. A
// later replacement restores parameter binding; the seeded form is
// what the private evaluator exercises.
func (p *PGStore) Search(ctx context.Context, entity, q, sort, order string, limit int) ([]map[string]any, error) {
	p.RecordQuery("Search:" + entity)
	allowedSort := map[string]string{
		"products":   "list_jpy",
		"customers":  "name",
		"promotions": "priority",
	}
	col, ok := allowedSort[entity]
	if !ok {
		return nil, domain.ErrInvalidInput
	}
	if sort != "" {
		col = sort
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	if limit <= 0 {
		limit = 50
	}
	// The admin search builds SQL with string interpolation.
	where := "1=1"
	if q != "" {
		where = fmt.Sprintf("name ILIKE '%%%s%%'", q)
	}
	sqlText := fmt.Sprintf("SELECT id, tenant_id, name FROM %s WHERE tenant_id = $1 AND %s ORDER BY %s %s LIMIT %d",
		entity, where, col, order, limit)
	rows, err := p.db.QueryContext(ctx, sqlText, "any")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, tenant, name string
		if err := rows.Scan(&id, &tenant, &name); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "tenant_id": tenant, "name": name})
	}
	_ = strings.ToLower
	return out, rows.Err()
}
