import { useEffect, useState } from "react"

import { getJSON } from "../api"

export default function OverviewPage() {
  const [status, setStatus] = useState(null)

  useEffect(() => {
    getJSON("/api/v1/status").then(setStatus).catch(() => setStatus(null))
  }, [])

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">INFRASTRUCTURE OVERVIEW</p>
          <h1>ReactorLab</h1>
          <p>
            Health and resource usage across the Dell, MiniDeploy, and MiniBase.
          </p>
        </div>
      </section>

      <section className="stats-grid">
        <article className="stat-card"><span>CPU</span><strong>—</strong></article>
        <article className="stat-card"><span>MEMORY</span><strong>—</strong></article>
        <article className="stat-card"><span>DISK</span><strong>—</strong></article>
        <article className="stat-card"><span>TEMPERATURE</span><strong>—</strong></article>
      </section>

      <section className="section-card">
        <div className="section-heading">
          <h2>Services</h2>
          <span className="status-badge status-ready">
            <span className="status-dot" />
            {status ? "Online" : "Checking"}
          </span>
        </div>

        <div className="service-list">
          <div className="service-row"><strong>ReactorLab</strong><span>Healthy</span></div>
          <div className="service-row"><strong>MiniDeploy</strong><span>Integration pending</span></div>
          <div className="service-row"><strong>MiniBase</strong><span>Integration pending</span></div>
          <div className="service-row"><strong>PostgreSQL</strong><span>Integration pending</span></div>
        </div>
      </section>
    </>
  )
}
