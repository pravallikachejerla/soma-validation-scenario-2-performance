import { useEffect, useState } from 'react';
import { api, Product } from '../lib/api';

export function ProductsPage() {
  const [rows, setRows] = useState<Product[]>([]);
  const [error, setError] = useState<string>('');

  useEffect(() => {
    api
      .search({ entity: 'products', sort: 'name', order: 'asc' })
      .then((r) => setRows(r.rows as Product[]))
      .catch((e) => setError(String(e)));
  }, []);

  return (
    <div>
      <h2>Products</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>SKU</th>
              <th>Name</th>
              <th>List (JPY)</th>
              <th>Category</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((p) => (
              <tr key={p.id}>
                <td>{p.sku}</td>
                <td>{p.name}</td>
                <td>{p.list_jpy}</td>
                <td>{p.category}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
