import {
  formatPercent,
  formatTemperature,
  formatUptime,
} from "../format"
import usePollingJSON from "../usePollingJSON"

export default function OverviewPage() {
  const { data: system, error, loading } = usePollingJSON("/api/v1/system")

  const activeServices =
    system?.services?.filter((service) => service.active).length ?? 0
  const serviceCount = system?.services?.length ?? 0

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">INFRASTRUCTURE OVERVIEW</p>
          <h1>ReactorLab</h1>
          <p>
            Live health and resource usage across the Dell infrastructure.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 5s"}</span>
        </div>
      </section>

      <section className="stats-grid">
        <article className="stat-card">
          <span>CPU</span>
          <strong>{formatPercent(system?.cpu?.usagePercent)}</strong>
          <small>{system?.cpu?.logicalCores ?? "—"} logical cores</small>
        </article>
        <article className="stat-card">
          <span>MEMORY</span>
          <strong>{formatPercent(system?.memory?.usagePercent)}</strong>
          <small>system RAM</small>
        </article>
        <article className="stat-card">
          <span>DISK</span>
          <strong>{formatPercent(system?.disk?.usagePercent)}</strong>
          <small>root filesystem</small>
        </article>
        <article className="stat-card">
          <span>TEMPERATURE</span>
          <strong>{formatTemperature(system?.temperature?.celsius)}</strong>
          <small>{system?.temperature?.source || "CPU sensor"}</small>
        </article>
      </section>

      {error ? <div className="notice error">{error}</div> : null}

      <section className="overview-grid">
        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">DELL</p>
              <h2>System health</h2>
            </div>
            <span className="status-badge status-ready">
              <span className="status-dot" />
              {loading ? "Checking" : "Online"}
            </span>
          </div>

          <div className="metric-lines">
            <div>
              <span>Uptime</span>
              <strong>{formatUptime(system?.uptimeSeconds)}</strong>
            </div>
            <div>
              <span>Load average</span>
              <strong>
                {system
                  ? `${system.cpu.load1.toFixed(2)} · ${system.cpu.load5.toFixed(2)} · ${system.cpu.load15.toFixed(2)}`
                  : "—"}
              </strong>
            </div>
            <div>
              <span>Services</span>
              <strong>
                {serviceCount ? `${activeServices}/${serviceCount} active` : "—"}
              </strong>
            </div>
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">SERVICES</p>
              <h2>Control plane</h2>
            </div>
          </div>

          <div className="service-list">
            {system?.services?.map((service) => (
              <div className="service-row" key={service.unit}>
                <strong>{service.name}</strong>
                <span className={service.active ? "service-up" : "service-down"}>
                  {service.active ? "Active" : service.status}
                </span>
              </div>
            )) || (
              <div className="service-row">
                <strong>ReactorLab</strong>
                <span>{loading ? "Checking" : "Unavailable"}</span>
              </div>
            )}
          </div>
        </article>
      </section>
    </>
  )
}
