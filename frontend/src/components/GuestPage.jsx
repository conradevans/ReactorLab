import { useEffect, useState } from "react"

import { getJSON } from "../api"
import Brand from "./Brand"
import ProductNav from "./ProductNav"

export default function GuestPage({ navigate }) {
  const [status, setStatus] = useState(null)

  useEffect(() => {
    getJSON("/api/v1/guest/status").then(setStatus).catch(() => setStatus(null))
  }, [])

  return (
    <main className="guest-page">
      <div className="site-shell">
        <header className="public-nav">
          <Brand navigate={navigate} subtitle="Guest infrastructure overview" />
          <div className="header-actions">
            <ProductNav mode="guest" />
            <button
              className="button secondary"
              type="button"
              onClick={() => navigate("/")}
            >
              Switch access
            </button>
          </div>
        </header>

        <section className="guest-hero">
          <div>
            <p className="eyebrow">GUEST OVERVIEW</p>
            <h1>ReactorLab health.</h1>
            <p>
              A restricted view of overall ReactorLab and Dell health.
              Detailed infrastructure information remains administrator-only.
            </p>
          </div>

          <div className="guest-summary">
            <div>
              <span>REACTORLAB</span>
              <strong>{status?.status === "ok" ? "Healthy" : "Unknown"}</strong>
            </div>
            <div>
              <span>ACCESS</span>
              <strong>Guest</strong>
            </div>
          </div>
        </section>

        <section className="content-section">
          <div className="section-card">
            <div className="section-heading">
              <h2>System overview</h2>
            </div>

            <div className="service-list">
              <div className="service-row"><strong>ReactorLab</strong><span>Healthy</span></div>
              <div className="service-row"><strong>MiniDeploy</strong><span>Monitoring pending</span></div>
              <div className="service-row"><strong>MiniBase</strong><span>Monitoring pending</span></div>
              <div className="service-row"><strong>Dell</strong><span>Metrics arriving in Phase 2</span></div>
            </div>
          </div>
        </section>

        <footer>ReactorLab · Restricted guest dashboard</footer>
      </div>
    </main>
  )
}
