import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, Routes, Route, NavLink, Navigate } from 'react-router-dom';
import { ProductsPage } from './pages/Products';
import { PromotionsPage } from './pages/Promotions';
import { SimulatorPage } from './pages/Simulator';
import { BatchPage } from './pages/Batch';
import { ExplanationPage } from './pages/Explanation';
import { AuditPage } from './pages/Audit';
import { AdminSearchPage } from './pages/AdminSearch';
import './app.css';

const nav = [
  { to: '/products', label: 'Products' },
  { to: '/promotions', label: 'Promotions' },
  { to: '/simulator', label: 'Simulator' },
  { to: '/batch', label: 'Batch' },
  { to: '/audit', label: 'Audit' },
  { to: '/admin-search', label: 'Admin Search' },
];

export function App() {
  return (
    <BrowserRouter>
      <header className="topbar">
        <h1>Pricing Console</h1>
        <nav>
          {nav.map((n) => (
            <NavLink key={n.to} to={n.to} className="navlink">
              {n.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="main">
        <Routes>
          <Route path="/" element={<Navigate to="/products" replace />} />
          <Route path="/products" element={<ProductsPage />} />
          <Route path="/promotions" element={<PromotionsPage />} />
          <Route path="/simulator" element={<SimulatorPage />} />
          <Route path="/batch" element={<BatchPage />} />
          <Route path="/decision/:id" element={<ExplanationPage />} />
          <Route path="/audit" element={<AuditPage />} />
          <Route path="/admin-search" element={<AdminSearchPage />} />
        </Routes>
      </main>
    </BrowserRouter>
  );
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
