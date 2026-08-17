// API client used by every page. The frontend talks to the Go API
// at the URL configured via VITE_API_BASE (default /api/v1).
const base = (import.meta as any).env?.VITE_API_BASE ?? '/api/v1';

export interface Product {
  id: string;
  tenant_id: string;
  sku: string;
  name: string;
  list_jpy: number;
  category?: string;
}

export interface Promotion {
  id: string;
  tenant_id: string;
  name: string;
  channel: string;
  product_scope: string;
  priority: number;
  percent_bp: number;
  valid_from: string;
  valid_to: string;
}

export interface AppliedPromotion {
  promotion_id: string;
  reason: string;
  amount_jpy: number;
}

export interface PricingDecision {
  id: string;
  request_id: string;
  tenant_id: string;
  sku: string;
  quantity: number;
  channel: string;
  list_jpy: number;
  base_jpy: number;
  subtotal_jpy: number;
  discount_jpy: number;
  amount_jpy: number;
  applied_promotion_ids: string[];
  applied: AppliedPromotion[];
  reason: string;
  mode: string;
  created_at: string;
}

export interface PricingRequest {
  tenant_id: string;
  customer_id: string;
  sku: string;
  quantity: number;
  channel: string;
}

export interface AuditEvent {
  id: string;
  tenant_id: string;
  action: string;
  entity: string;
  entity_id: string;
  notes: string;
  created_at: string;
}

async function http<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(base + path, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!r.ok) {
    const text = await r.text();
    throw new Error(`HTTP ${r.status}: ${text}`);
  }
  return (await r.json()) as T;
}

export const api = {
  quote: (req: PricingRequest) =>
    http<PricingDecision>('/pricing/quote', { method: 'POST', body: JSON.stringify(req) }),
  batch: (req: { tenant_id: string; items: PricingRequest[] }) =>
    http<{ decisions: PricingDecision[] }>('/pricing/batch', {
      method: 'POST',
      body: JSON.stringify(req),
    }),
  promotions: (tenantId: string) =>
    http<{ promotions: Promotion[] }>(`/promotions?tenant_id=${encodeURIComponent(tenantId)}`),
  createPromotion: (p: Partial<Promotion>) =>
    http<Promotion>('/promotions', { method: 'POST', body: JSON.stringify(p) }),
  search: (params: { entity: string; q?: string; sort?: string; order?: string }) => {
    const qp = new URLSearchParams();
    qp.set('entity', params.entity);
    if (params.q) qp.set('q', params.q);
    if (params.sort) qp.set('sort', params.sort);
    if (params.order) qp.set('order', params.order);
    return http<{ rows: any[] }>(`/admin/search?${qp.toString()}`);
  },
  audit: (tenantId: string) =>
    http<{ events: AuditEvent[] }>(`/admin/audit?tenant_id=${encodeURIComponent(tenantId)}`),
};
