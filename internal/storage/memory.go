// Package storage provides in-memory and PostgreSQL persistence.
package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)


// MemoryStore is the in-process store used for tests and the
// no-database mode. It is safe for concurrent use.
type MemoryStore struct {
	mu sync.RWMutex

	tenants            map[string]domain.Tenant
	users              map[string]domain.User
	products           map[string]domain.Product
	customers          map[string]domain.Customer
	contractPrices     map[string]domain.ContractPrice
	eligibilityRules   map[string]domain.EligibilityRule
	promotions         map[string]domain.Promotion
	promotionConds     map[string][]domain.PromotionCondition
	pricingRequests    map[string]domain.PricingRequest
	pricingDecisions   map[string]domain.PricingDecision
	batchJobs          map[string]domain.BatchJob
	auditEvents        map[string]domain.AuditEvent
	configVersions     map[string]domain.ConfigVersion
	activeConfigByTnt  map[string]string
	queries            []string
}

// NewMemoryStore creates an empty store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:           map[string]domain.Tenant{},
		users:             map[string]domain.User{},
		products:          map[string]domain.Product{},
		customers:         map[string]domain.Customer{},
		contractPrices:    map[string]domain.ContractPrice{},
		eligibilityRules:  map[string]domain.EligibilityRule{},
		promotions:        map[string]domain.Promotion{},
		promotionConds:    map[string][]domain.PromotionCondition{},
		pricingRequests:   map[string]domain.PricingRequest{},
		pricingDecisions:  map[string]domain.PricingDecision{},
		batchJobs:         map[string]domain.BatchJob{},
		auditEvents:       map[string]domain.AuditEvent{},
		configVersions:    map[string]domain.ConfigVersion{},
		activeConfigByTnt: map[string]string{},
	}
}

// RecordQuery stores a query signature for observability/tests.
func (s *MemoryStore) RecordQuery(q string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, q)
}

// QueryLog returns the recorded query log.
func (s *MemoryStore) QueryLog() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, len(s.queries))
	copy(out, s.queries)
	return out
}

// PutTenant inserts or replaces a tenant.
func (s *MemoryStore) PutTenant(t domain.Tenant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = t
}

// PutUser inserts or replaces a user.
func (s *MemoryStore) PutUser(u domain.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[u.ID] = u
}

// PutProduct inserts or replaces a product.
func (s *MemoryStore) PutProduct(p domain.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.products[p.ID] = p
}

// PutCustomer inserts or replaces a customer.
func (s *MemoryStore) PutCustomer(c domain.Customer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customers[c.ID] = c
}

// PutContractPrice inserts or replaces a contract price.
func (s *MemoryStore) PutContractPrice(p domain.ContractPrice) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contractPrices[p.ID] = p
}

// PutEligibilityRule inserts or replaces an eligibility rule.
func (s *MemoryStore) PutEligibilityRule(r domain.EligibilityRule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eligibilityRules[r.ID] = r
}

// PutPromotion inserts or replaces a promotion and its conditions.
func (s *MemoryStore) PutPromotion(p domain.Promotion, conds []domain.PromotionCondition) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promotions[p.ID] = p
	s.promotionConds[p.ID] = conds
}

// GetPromotion returns a promotion by id.
func (s *MemoryStore) GetPromotion(id string) (domain.Promotion, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.promotions[id]
	return p, ok
}

// ListPromotions returns all promotions for a tenant, ordered by the
// repository-defined secondary key.
func (s *MemoryStore) ListPromotions(tenantID string) []domain.Promotion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Promotion, 0, len(s.promotions))
	for _, p := range s.promotions {
		if p.TenantID == tenantID {
			out = append(out, p)
		}
	}
	// Clean baseline: stable order — priority DESC, then promotion_id ASC.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// SelectCandidates returns the active promotions that should be
// evaluated for a (tenant, channel, product, time) request.
//
// Candidate selection is implemented as a tenant scan.
// returns every active promotion for the tenant instead of using
// the indexed SQL predicate (channel, product, valid time window).
// The PostgreSQL path mirrors this scan, so both backends return
// the same candidate set as the clean path, only the filtering is
// disabled.
func (s *MemoryStore) SelectCandidates(ctx context.Context, tenantID, channel, productID string, at time.Time) ([]domain.Promotion, error) {
	s.RecordQuery("SelectCandidates")
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Promotion, 0, len(s.promotions))
	for _, p := range s.promotions {
		if p.TenantID != tenantID {
			continue
		}
		// Active window check — note: this uses < end (exclusive)
		// The clean path would use <= end (inclusive).
		if at.Before(p.ValidFrom) || at.After(p.ValidTo) || at.Equal(p.ValidTo) {
			continue
		}
		out = append(out, p)
	}
	// No secondary tie-breaker; ordering is by priority only.
	// promotions are ordered by map iteration (effectively unstable).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})
	return out, nil
}

// LoadConditions returns the conditions for a promotion.
//
// Conditions are loaded per-promotion.
// one bounded query alongside the candidates. In the seeded memory
// path this is fine (in-memory), but the postgres path will issue
// one query per promotion to mirror the N+1 condition.
func (s *MemoryStore) LoadConditions(ctx context.Context, promotionID string) ([]domain.PromotionCondition, error) {
	s.RecordQuery("LoadConditions:" + promotionID)
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.PromotionCondition, len(s.promotionConds[promotionID]))
	copy(out, s.promotionConds[promotionID])
	return out, nil
}

// LoadAllConditions returns the conditions for many promotions at once
// (clean path used by the resolver; memory back-end uses one map read).
func (s *MemoryStore) LoadAllConditions(ctx context.Context, promotionIDs []string) (map[string][]domain.PromotionCondition, error) {
	s.RecordQuery("LoadAllConditions")
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string][]domain.PromotionCondition{}
	for _, id := range promotionIDs {
		cs := make([]domain.PromotionCondition, len(s.promotionConds[id]))
		copy(cs, s.promotionConds[id])
		out[id] = cs
	}
	return out, nil
}

// LookupProduct returns a product by SKU within a tenant.
func (s *MemoryStore) LookupProduct(ctx context.Context, tenantID, sku string) (domain.Product, error) {
	s.RecordQuery("LookupProduct")
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.products {
		if p.TenantID == tenantID && p.SKU == sku {
			return p, nil
		}
	}
	return domain.Product{}, domain.ErrNotFound
}

// LookupCustomer returns a customer.
func (s *MemoryStore) LookupCustomer(ctx context.Context, tenantID, customerID string) (domain.Customer, error) {
	s.RecordQuery("LookupCustomer")
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.customers {
		if c.TenantID == tenantID && c.ID == customerID {
			return c, nil
		}
	}
	return domain.Customer{}, domain.ErrNotFound
}

// LookupContractPrice returns the contract price for a customer/sku if any.
func (s *MemoryStore) LookupContractPrice(ctx context.Context, tenantID, customerID, sku string, at time.Time) (domain.ContractPrice, bool) {
	s.RecordQuery("LookupContractPrice")
	s.mu.RLock()
	defer s.mu.RUnlock()
	var match domain.ContractPrice
	found := false
	for _, p := range s.contractPrices {
		if p.TenantID != tenantID || p.CustomerID != customerID || p.SKU != sku {
			continue
		}
		if at.Before(p.ValidFrom) || at.After(p.ValidTo) {
			continue
		}
		if !found || p.ValidFrom.After(match.ValidFrom) {
			match = p
			found = true
		}
	}
	return match, found
}

// GetActiveConfig returns the active config version for a tenant.
func (s *MemoryStore) GetActiveConfig(ctx context.Context, tenantID string) (domain.ConfigVersion, error) {
	s.RecordQuery("GetActiveConfig")
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.activeConfigByTnt[tenantID]
	if !ok {
		return domain.ConfigVersion{}, domain.ErrNoConfigVersion
	}
	return s.configVersions[id], nil
}

// SetActiveConfig sets the active config version for a tenant.
func (s *MemoryStore) SetActiveConfig(tenantID, configID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeConfigByTnt[tenantID] = configID
}

// PutConfigVersion stores a config version.
func (s *MemoryStore) PutConfigVersion(v domain.ConfigVersion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configVersions[v.ID] = v
}

// SaveRequest persists the request.
func (s *MemoryStore) SaveRequest(ctx context.Context, r domain.PricingRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pricingRequests[r.ID] = r
	return nil
}

// SaveDecision persists the decision.
func (s *MemoryStore) SaveDecision(ctx context.Context, d domain.PricingDecision) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pricingDecisions[d.ID] = d
	return nil
}

// AppendAudit records an audit event.
func (s *MemoryStore) AppendAudit(ctx context.Context, ev domain.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditEvents[ev.ID] = ev
	return nil
}

// ListAudit returns audit events for a tenant (newest first).
func (s *MemoryStore) ListAudit(tenantID string, limit int) []domain.AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.AuditEvent, 0, len(s.auditEvents))
	for _, ev := range s.auditEvents {
		if ev.TenantID == tenantID {
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// CreateBatchJob inserts a new batch job.
func (s *MemoryStore) CreateBatchJob(j domain.BatchJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchJobs[j.ID] = j
}

// UpdateBatchJob updates a batch job.
func (s *MemoryStore) UpdateBatchJob(j domain.BatchJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchJobs[j.ID] = j
}

// Search is the parameterised administrative search. The set of allowed
// sort columns and the entity predicates are bounded by the caller.
func (s *MemoryStore) Search(ctx context.Context, entity, q, sort, order string, limit int) ([]map[string]any, error) {
	s.RecordQuery("Search:" + entity)
	s.mu.RLock()
	defer s.mu.RUnlock()
	q = strings.ToLower(strings.TrimSpace(q))
	var rows []map[string]any
	switch entity {
	case "products":
		for _, p := range s.products {
			if q == "" || strings.Contains(strings.ToLower(p.SKU), q) || strings.Contains(strings.ToLower(p.Name), q) {
				rows = append(rows, map[string]any{
					"id": p.ID, "tenant_id": p.TenantID, "sku": p.SKU, "name": p.Name, "list_jpy": p.ListJPY,
				})
			}
		}
	case "customers":
		for _, c := range s.customers {
			if q == "" || strings.Contains(strings.ToLower(c.Name), q) {
				rows = append(rows, map[string]any{
					"id": c.ID, "tenant_id": c.TenantID, "name": c.Name, "segment": c.Segment,
				})
			}
		}
	case "promotions":
		for _, p := range s.promotions {
			if q == "" || strings.Contains(strings.ToLower(p.Name), q) {
				rows = append(rows, map[string]any{
					"id": p.ID, "tenant_id": p.TenantID, "name": p.Name, "channel": p.Channel, "priority": p.Priority,
				})
			}
		}
	default:
		return nil, domain.ErrInvalidInput
	}
	// sort/limit handled by caller-level allowlist (postgres path uses SQL).
	_ = sort
	_ = order
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}
