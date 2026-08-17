import { useState } from 'react';
import { api, PricingDecision, PricingRequest } from '../lib/api';

export function BatchPage() {
  const [tenantId, setTenantId] = useState('tenant-a');
  const [lines, setLines] = useState('SKU-tenant-a-00001,cust-tenant-a-00001,1,web\nSKU-tenant-a-00002,cust-tenant-a-00002,2,web');
  const [decisions, setDecisions] = useState<PricingDecision[]>([]);
  const [error, setError] = useState<string>('');

  async function run() {
    try {
      const items: PricingRequest[] = lines
        .split('\n')
        .filter(Boolean)
        .map((l) => {
          const [sku, customer_id, quantity, channel] = l.split(',');
          return {
            tenant_id: tenantId,
            sku: sku.trim(),
            customer_id: customer_id.trim(),
            quantity: Number(quantity),
            channel: channel.trim(),
          };
        });
      const out = await api.batch({ tenant_id: tenantId, items });
      setDecisions(out.decisions);
    } catch (e) {
      setError(String(e));
    }
  }

  return (
    <div>
      <h2>Batch Pricing</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <label>
          Tenant
          <input value={tenantId} onChange={(e) => setTenantId(e.target.value)} />
        </label>
        <label>
          Items (sku,customer_id,quantity,channel — one per line)
          <textarea value={lines} onChange={(e) => setLines(e.target.value)} rows={5} />
        </label>
        <button onClick={run}>Run</button>
      </div>
      {decisions.length > 0 && (
        <div className="card">
          <table>
            <thead>
              <tr>
                <th>Decision</th>
                <th>SKU</th>
                <th>Qty</th>
                <th>Subtotal</th>
                <th>Discount</th>
                <th>Amount</th>
                <th>Applied</th>
              </tr>
            </thead>
            <tbody>
              {decisions.map((d) => (
                <tr key={d.id}>
                  <td>{d.id}</td>
                  <td>{d.sku}</td>
                  <td>{d.quantity}</td>
                  <td>{d.subtotal_jpy}</td>
                  <td>{d.discount_jpy}</td>
                  <td>{d.amount_jpy}</td>
                  <td>{d.applied_promotion_ids.join(', ')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
