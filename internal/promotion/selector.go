package promotion

import (
	"context"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// CandidateStore is the storage surface used by the pricing engine.
// It mirrors the methods on storage.MemoryStore and storage.PGStore.
type CandidateStore interface {
	SelectCandidates(ctx context.Context, tenantID, channel, productID string, at time.Time) ([]domain.Promotion, error)
	LoadAllConditions(ctx context.Context, promotionIDs []string) (map[string][]domain.PromotionCondition, error)
	LoadConditions(ctx context.Context, promotionID string) ([]domain.PromotionCondition, error)
}
