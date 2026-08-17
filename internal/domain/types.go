// Package domain contains the core entity types shared between layers.
// These types are the source of truth for the pricing application.
package domain

import "time"

// Tenant is a logical isolation boundary for data and configuration.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// User belongs to a tenant with a specific role.
type User struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// Product is a sellable SKU.
type Product struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	ListJPY  int64  `json:"list_jpy"`
	Category string `json:"category"`
}

// Customer is an account that can be priced.
type Customer struct {
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Segment  string `json:"segment"`
}

// ContractPrice is a customer-specific base price override.
type ContractPrice struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	CustomerID string    `json:"customer_id"`
	SKU        string    `json:"sku"`
	BaseJPY    int64     `json:"base_jpy"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidTo    time.Time `json:"valid_to"`
}

// EligibilityRule restricts where promotions may apply.
type EligibilityRule struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Channel    string    `json:"channel"`
	Segment    string    `json:"segment"`
	Category   string    `json:"category"`
	ValidFrom  time.Time `json:"valid_from"`
	ValidTo    time.Time `json:"valid_to"`
}

// Promotion is a discount that may apply to a pricing request.
type Promotion struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Name         string    `json:"name"`
	Channel      string    `json:"channel"`
	ProductID    string    `json:"product_id,omitempty"`
	ProductScope string    `json:"product_scope"` // "sku" or "wildcard"
	Priority     int       `json:"priority"`
	Stacking     bool      `json:"stacking"`
	Exclusion    bool      `json:"exclusion"`
	PercentBP    int       `json:"percent_bp"` // basis points (e.g. 500 = 5%)
	ValidFrom    time.Time `json:"valid_from"`
	ValidTo      time.Time `json:"valid_to"`
}

// PromotionCondition is an additional constraint (e.g. min quantity).
type PromotionCondition struct {
	ID          string `json:"id"`
	PromotionID string `json:"promotion_id"`
	Kind        string `json:"kind"`
	Expr        string `json:"expr"`
}

// PricingRequest captures the inputs to a quote.
type PricingRequest struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	CustomerID string    `json:"customer_id"`
	SKU        string    `json:"sku"`
	Quantity   int       `json:"quantity"`
	Channel    string    `json:"channel"`
	OccurredAt time.Time `json:"occurred_at"`
}

// AppliedPromotion is the audit record of one promotion on a decision.
type AppliedPromotion struct {
	PromotionID string `json:"promotion_id"`
	Reason      string `json:"reason"`
	AmountJPY   int64  `json:"amount_jpy"`
}

// PricingDecision is the result of evaluating a request.
type PricingDecision struct {
	ID                   string             `json:"id"`
	RequestID            string             `json:"request_id"`
	TenantID             string             `json:"tenant_id"`
	CustomerID           string             `json:"customer_id"`
	SKU                  string             `json:"sku"`
	Quantity             int                `json:"quantity"`
	Channel              string             `json:"channel"`
	ListJPY              int64              `json:"list_jpy"`
	BaseJPY              int64              `json:"base_jpy"`
	SubtotalJPY          int64              `json:"subtotal_jpy"`
	DiscountJPY          int64              `json:"discount_jpy"`
	AmountJPY            int64              `json:"amount_jpy"`
	AppliedPromotionIDs  []string           `json:"applied_promotion_ids"`
	Applied              []AppliedPromotion `json:"applied"`
	Reason               string             `json:"reason"`
	ConfigVersion        string             `json:"config_version"`
	Mode                 string             `json:"mode"` // "interactive" | "batch"
	CreatedAt            time.Time          `json:"created_at"`
}

// BatchJob is a collection of pricing requests processed together.
type BatchJob struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	TotalItems  int       `json:"total_items"`
	DoneItems   int       `json:"done_items"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// AuditEvent records a material change.
type AuditEvent struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	ActorID   string    `json:"actor_id"`
	Action    string    `json:"action"`
	Entity    string    `json:"entity"`
	EntityID  string    `json:"entity_id"`
	RequestID string    `json:"request_id,omitempty"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfigVersion identifies a frozen rule set.
type ConfigVersion struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Label     string    `json:"label"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}
