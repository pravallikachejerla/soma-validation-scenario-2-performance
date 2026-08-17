import { useState } from 'react';
import { api } from '../lib/api';

export function AdminSearchPage() {
  const [entity, setEntity] = useState('products');
  const [q, setQ] = useState('');
  const [sort, setSort] = useState('name');
  const [order, setOrder] = useState('asc');
  const [rows, setRows] = useState<any[]>([]);
  const [error, setError] = useState<string>('');

  async function run() {
    try {
      const r = await api.search({ entity, q, sort, order });
      setRows(r.rows);
    } catch (e) {
      setError(String(e));
    }
  }

  return (
    <div>
      <h2>Admin Search</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <div className="row">
          <label>
            Entity
            <select value={entity} onChange={(e) => setEntity(e.target.value)}>
              <option value="products">products</option>
              <option value="customers">customers</option>
              <option value="promotions">promotions</option>
            </select>
          </label>
          <label>
            Query
            <input value={q} onChange={(e) => setQ(e.target.value)} />
          </label>
          <label>
            Sort
            <input value={sort} onChange={(e) => setSort(e.target.value)} />
          </label>
          <label>
            Order
            <select value={order} onChange={(e) => setOrder(e.target.value)}>
              <option value="asc">asc</option>
              <option value="desc">desc</option>
            </select>
          </label>
        </div>
        <button onClick={run}>Search</button>
      </div>
      {rows.length > 0 && (
        <div className="card">
          <table>
            <thead>
              <tr>
                <th>ID</th>
                <th>Tenant</th>
                <th>Name</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.id}>
                  <td>{r.id}</td>
                  <td>{r.tenant_id}</td>
                  <td>{r.name}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
