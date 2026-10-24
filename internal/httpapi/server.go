// Package httpapi provides the HTTP handlers and routing for the
// pricing application.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/adminsearch"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/application"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/observability"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/seeddata"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/security"
	"github.com/soma-genesis/scenario-2-pricing-perf/internal/validation"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)


// Server is the HTTP server.
type Server struct {
	app  *application.App
	mux  *chi.Mux
	sink io.Writer
}

// New returns a new server.
func New(app *application.App) *Server {
	s := &Server{app: app, mux: chi.NewRouter(), sink: os.Stdout}
	s.routes()
	return s
}

// ServeHTTP makes Server an http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Mux exposes the chi router for direct use in tests.
func (s *Server) Mux() *chi.Mux { return s.mux }

func (s *Server) routes() {
	s.mux.Use(s.requestID)
	s.mux.Use(s.accessLog)

	s.mux.Get("/healthz", s.handleHealthz)
	s.mux.Get("/version", s.handleVersion)
	s.mux.Handle("/metrics", s.app.Metrics.Handler())

	s.mux.Route("/api/v1", func(r chi.Router) {
		r.Post("/pricing/quote", s.handleQuote)
		r.Post("/pricing/batch", s.handleBatch)
		r.Get("/promotions", s.handleListPromotions)
		r.Post("/promotions", s.handleCreatePromotion)
		r.Patch("/promotions/{id}", s.handlePatchPromotion)
		r.Get("/admin/search", s.handleAdminSearch)
		r.Get("/admin/audit", s.handleAudit)
		r.Post("/admin/seed", s.handleSeed)
	})
}

// requestID middleware attaches a request id to the context.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// accessLog records structured per-request logs.
//
// The access log carries unredacted request body fields.
// fields (customer_id, negotiated_price, discount_reason) WITHOUT
// redaction. The clean path replaces them with synthetic IDs.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)

		reqID, _ := r.Context().Value(requestIDKey{}).(string)

		// The access log carries unredacted request body fields.
		// The seeded access logger writes the customer identifier,
		// negotiated price and discount reason to the structured
		// log without redaction. The clean path replaces these
		// with synthetic-safe identifiers.
		var logBody any
		if r.URL.Path == "/api/v1/pricing/quote" {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			var parsed struct {
				CustomerID      string `json:"customer_id"`
				NegotiatedPrice int64  `json:"negotiated_price"`
				DiscountReason  string `json:"discount_reason"`
			}
			_ = json.Unmarshal(body, &parsed)
			logBody = map[string]any{
				"customer_id":      parsed.CustomerID,
				"negotiated_price": parsed.NegotiatedPrice,
				"discount_reason":  parsed.DiscountReason,
			}
		}

		rec := map[string]any{
			"ts":         time.Now().UTC().Format(time.RFC3339Nano),
			"level":      "info",
			"msg":        "http_request",
			"request_id": reqID,
			"route":      r.URL.Path,
			"method":     r.Method,
			"status":     ww.status,
			"latency_ms": time.Since(start).Milliseconds(),
		}
		if logBody != nil {
			rec["body"] = logBody
		}
		b, _ := json.Marshal(rec)
		_, _ = s.sink.Write(append(b, '\n'))
	})
}

type requestIDKey struct{}
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"mode":   s.app.Mode,
		"ts":     time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Version)
}

func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	body, err := readBoundedBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var req domain.PricingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := validation.ValidatePricingRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !validation.IsValidChannel(req.Channel) {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown channel: %s", req.Channel))
		return
	}
	if req.ID == "" {
		req.ID = uuid.NewString()
	}
	dec, err := s.app.Engine.Evaluate(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, dec)
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	body, err := readBoundedBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var payload struct {
		TenantID string                 `json:"tenant_id"`
		Items    []domain.PricingRequest `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if payload.TenantID == "" || len(payload.Items) == 0 {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	decisions, err := s.app.Engine.ProcessBatch(r.Context(), payload.Items)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"decisions": decisions,
		"summary":   map[string]any{"count": len(decisions)},
	})
}

func (s *Server) handleListPromotions(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	channel := r.URL.Query().Get("channel")
	dateStr := r.URL.Query().Get("date")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	at := time.Now().UTC()
	if dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			at = t
		}
	}
	_ = channel
	all := s.listPromotions(tenantID)
	writeJSON(w, http.StatusOK, map[string]any{"promotions": all, "as_of": at})
}

func (s *Server) handleCreatePromotion(w http.ResponseWriter, r *http.Request) {
	body, err := readBoundedBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var p domain.Promotion
	if err := json.Unmarshal(body, &p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.TenantID == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	if s.app.MemStore != nil {
		s.app.MemStore.PutPromotion(p, nil)
	}
	if err := s.appendAudit(r, p.TenantID, "promotion.create", "promotion", p.ID, ""); err != nil {
		observability.Warn("audit failed", "err", err)
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePatchPromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if s.app.MemStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("memory store only in this build"))
		return
	}
	p, ok := s.app.MemStore.GetPromotion(id)
	if !ok {
		writeError(w, http.StatusNotFound, domain.ErrNotFound)
		return
	}
	body, err := readBoundedBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var patch map[string]any
	if err := json.Unmarshal(body, &patch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if v, ok := patch["priority"].(float64); ok {
		p.Priority = int(v)
	}
	if v, ok := patch["percent_bp"].(float64); ok {
		p.PercentBP = int(v)
	}
	if v, ok := patch["stacking"].(bool); ok {
		p.Stacking = v
	}
	s.app.MemStore.PutPromotion(p, nil)
	if err := s.appendAudit(r, p.TenantID, "promotion.update", "promotion", p.ID, ""); err != nil {
		observability.Warn("audit failed", "err", err)
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleAdminSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	entity := q.Get("entity")
	term := q.Get("q")
	sort := q.Get("sort")
	order := q.Get("order")
	limit := validation.ClampLimit(parseInt(q.Get("limit")), 50, 500)

	if s.app.Mode == "postgres" && s.app.PGStore != nil {
		rows, err := s.app.PGStore.Search(r.Context(), entity, term, sort, order, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
		return
	}
	if s.app.MemStore == nil {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("no store"))
		return
	}
	rows, err := s.app.MemStore.Search(r.Context(), entity, term, sort, order, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
	// Build the SQL trace for observability and tests.
	sqlText, _, _ := adminsearch.BuildQuery(entity, term, sort, order, limit)
	observability.Info("adminsearch.sql", "sql", sqlText, "entity", entity, "q", term, "sort", sort)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, domain.ErrInvalidInput)
		return
	}
	limit := validation.ClampLimit(parseInt(r.URL.Query().Get("limit")), 100, 1000)
	if s.app.MemStore != nil {
		writeJSON(w, http.StatusOK, map[string]any{"events": s.app.MemStore.ListAudit(tenantID, limit)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": []any{}})
}

func (s *Server) handleSeed(w http.ResponseWriter, r *http.Request) {
	profile := seeddata.Small
	ds := seeddata.Build(int64(seeddata.DefaultSeed), profile)
	written := map[string]int{
		"tenants":     len(ds.TenantIDs),
		"products":    len(ds.Products),
		"customers":   len(ds.Customers),
		"promotions":  len(ds.Promotions),
		"conditions":  len(ds.Conditions),
	}
	// Populate in-memory store when present.
	if s.app.MemStore != nil {
		for _, t := range ds.TenantIDs {
			s.app.MemStore.PutTenant(domain.Tenant{ID: t, Name: t, CreatedAt: time.Now().UTC()})
		}
		for _, p := range ds.Products {
			s.app.MemStore.PutProduct(p)
		}
		for _, c := range ds.Customers {
			s.app.MemStore.PutCustomer(c)
		}
		condsByPromo := map[string][]domain.PromotionCondition{}
		for _, c := range ds.Conditions {
			condsByPromo[c.PromotionID] = append(condsByPromo[c.PromotionID], c)
		}
		for _, p := range ds.Promotions {
			s.app.MemStore.PutPromotion(p, condsByPromo[p.ID])
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": profile.Name, "counts": written})
}

func (s *Server) listPromotions(tenantID string) []domain.Promotion {
	if s.app.MemStore == nil {
		return nil
	}
	return s.app.MemStore.ListPromotions(tenantID)
}

func (s *Server) appendAudit(r *http.Request, tenantID, action, entity, entityID, requestID string) error {
	if s.app.MemStore == nil {
		return nil
	}
	ev := domain.AuditEvent{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		ActorID:   security.Hash("system"),
		Action:    action,
		Entity:    entity,
		EntityID:  entityID,
		RequestID: requestID,
		Notes:     "",
		CreatedAt: time.Now().UTC(),
	}
	_ = s.app.MemStore.AppendAudit(r.Context(), ev)
	return nil
}

func readBoundedBody(r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, validation.MaxRequestBodyBytes)
	return io.ReadAll(r.Body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Suppress unused import warning when file re-ordered.
var _ = strings.TrimSpace
