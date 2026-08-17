import { useEffect, useState } from 'react';
import { api, Promotion } from '../lib/api';

export function PromotionsPage() {
  const [tenantId, setTenantId] = useState('tenant-a');
  const [rows, setRows] = useState<Promotion[]>([]);
  const [error, setError] = useState<string>('');
  const [draft, setDraft] = useState<Partial<Promotion>>({
    tenant_id: 'tenant-a',
    name: '',
    channel: 'web',
    product_scope: 'wildcard',
    priority: 1,
    percent_bp: 500,
    valid_from: '2026-01-01T00:00:00Z',
    valid_to: '2026-12-31T00:00:00Z',
  });

  useEffect(() => {
    api
      .promotions(tenantId)
      .then((r) => setRows(r.promotions))
      .catch((e) => setError(String(e)));
  }, [tenantId]);

  async function save() {
    try {
      const p = await api.createPromotion(draft);
      setRows([...rows, p]);
    } catch (e) {
      setError(String(e));
    }
  }

  return (
    <div>
      <h2>Promotions</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <div className="row">
          <label>
            Tenant
            <input value={tenantId} onChange={(e) => setTenantId(e.target.value)} />
          </label>
          <label>
            Channel
            <input value={draft.channel} onChange={(e) => setDraft({ ...draft, channel: e.target.value })} />
          </label>
          <label>
            Name
            <input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />
          </label>
          <label>
            Percent (basis points)
            <input
              type="number"
              value={draft.percent_bp}
              onChange={(e) => setDraft({ ...draft, percent_bp: Number(e.target.value) })}
            />
          </label>
          <label>
            Priority
            <input
              type="number"
              value={draft.priority}
              onChange={(e) => setDraft({ ...draft, priority: Number(e.target.value) })}
            />
          </label>
        </div>
        <button onClick={save}>Save promotion</button>
      </div>
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Name</th>
              <th>Channel</th>
              <th>Priority</th>
              <th>Percent BP</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((p) => (
              <tr key={p.id}>
                <td>{p.id}</td>
                <td>{p.name}</td>
                <td>{p.channel}</td>
                <td>{p.priority}</td>
                <td>{p.percent_bp}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
