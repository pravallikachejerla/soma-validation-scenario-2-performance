import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, PricingDecision, PricingRequest } from '../lib/api';

export function SimulatorPage() {
  const nav = useNavigate();
  const [req, setReq] = useState<PricingRequest>({
    tenant_id: 'tenant-a',
    customer_id: 'cust-tenant-a-00001',
    sku: 'SKU-tenant-a-00001',
    quantity: 1,
    channel: 'web',
  });
  const [result, setResult] = useState<PricingDecision | null>(null);
  const [error, setError] = useState<string>('');

  async function run() {
    try {
      const d = await api.quote(req);
      setResult(d);
      localStorage.setItem('lastDecision', JSON.stringify(d));
      nav(`/decision/${d.id}`);
    } catch (e) {
      setError(String(e));
    }
  }

  return (
    <div>
      <h2>Pricing Simulator</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <div className="row">
          <label>
            Tenant
            <input value={req.tenant_id} onChange={(e) => setReq({ ...req, tenant_id: e.target.value })} />
          </label>
          <label>
            Customer
            <input value={req.customer_id} onChange={(e) => setReq({ ...req, customer_id: e.target.value })} />
          </label>
          <label>
            SKU
            <input value={req.sku} onChange={(e) => setReq({ ...req, sku: e.target.value })} />
          </label>
          <label>
            Quantity
            <input
              type="number"
              value={req.quantity}
              onChange={(e) => setReq({ ...req, quantity: Number(e.target.value) })}
            />
          </label>
          <label>
            Channel
            <select value={req.channel} onChange={(e) => setReq({ ...req, channel: e.target.value })}>
              <option value="web">web</option>
              <option value="store">store</option>
              <option value="partner">partner</option>
              <option value="api">api</option>
              <option value="wholesale">wholesale</option>
            </select>
          </label>
        </div>
        <button onClick={run}>Quote</button>
      </div>
      {result && (
        <div className="card">
          <h3>Result</h3>
          <div className="muted">
            Subtotal: {result.subtotal_jpy} JPY · Discount: {result.discount_jpy} JPY · Amount:{' '}
            {result.amount_jpy} JPY
          </div>
          <ul>
            {result.applied.map((a) => (
              <li key={a.promotion_id}>
                {a.promotion_id} · {a.reason} · {a.amount_jpy} JPY
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
