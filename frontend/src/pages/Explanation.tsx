import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { PricingDecision } from '../lib/api';

export function ExplanationPage() {
  const { id } = useParams<{ id: string }>();
  const [dec, setDec] = useState<PricingDecision | null>(null);

  useEffect(() => {
    // The pricing decision is held in memory by the in-memory store
    // and not directly retrievable through a GET endpoint, so we
    // surface the most recent decision from the simulator by
    // reading it from a client-side cache.
    const cached = localStorage.getItem('lastDecision');
    if (cached) {
      setDec(JSON.parse(cached) as PricingDecision);
    }
  }, [id]);

  if (!dec) {
    return (
      <div className="card">
        <p>No decision available. Run the simulator first.</p>
      </div>
    );
  }
  return (
    <div>
      <h2>Decision {dec.id}</h2>
      <div className="card">
        <div className="muted">Mode: {dec.mode}</div>
        <div>Subtotal: {dec.subtotal_jpy} JPY</div>
        <div>Discount: {dec.discount_jpy} JPY</div>
        <div>Amount: {dec.amount_jpy} JPY</div>
      </div>
      <div className="card">
        <h3>Applied promotions</h3>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Reason</th>
              <th>Amount</th>
            </tr>
          </thead>
          <tbody>
            {dec.applied.map((a) => (
              <tr key={a.promotion_id}>
                <td>{a.promotion_id}</td>
                <td>{a.reason}</td>
                <td>{a.amount_jpy}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
