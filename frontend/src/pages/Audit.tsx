import { useEffect, useState } from 'react';
import { api, AuditEvent } from '../lib/api';

export function AuditPage() {
  const [tenantId, setTenantId] = useState('tenant-a');
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [error, setError] = useState<string>('');

  useEffect(() => {
    api
      .audit(tenantId)
      .then((r) => setEvents(r.events))
      .catch((e) => setError(String(e)));
  }, [tenantId]);

  return (
    <div>
      <h2>Audit Viewer</h2>
      {error && <div className="card muted">Error: {error}</div>}
      <div className="card">
        <label>
          Tenant
          <input value={tenantId} onChange={(e) => setTenantId(e.target.value)} />
        </label>
      </div>
      <div className="card">
        <table>
          <thead>
            <tr>
              <th>When</th>
              <th>Action</th>
              <th>Entity</th>
              <th>Notes</th>
            </tr>
          </thead>
          <tbody>
            {events.map((e) => (
              <tr key={e.id}>
                <td>{e.created_at}</td>
                <td>{e.action}</td>
                <td>{e.entity}</td>
                <td>{e.notes}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
