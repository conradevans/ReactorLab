import {
  formatBytes,
  formatPercent,
  formatRate,
  formatTemperature,
  formatUptime,
} from "../format"
import usePollingJSON from "../usePollingJSON"

function Metric({ label, value, detail }) {
  return (
    <div className="metric-detail">
      <span>{label}</span>
      <strong>{value}</strong>
      {detail ? <small>{detail}</small> : null}
    </div>
  )
}

export default function SystemPage() {
  const { data: system, error, loading } = usePollingJSON("/api/v1/system")

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">SYSTEM</p>
          <h1>Dell system</h1>
          <p>
            Live host resource usage, networking, uptime, and ReactorLab service
            health.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 5s"}</span>
        </div>
      </section>

      {error ? <div className="notice error">{error}</div> : null}

      <section className="system-section-grid">
        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">PROCESSOR</p>
              <h2>CPU</h2>
            </div>
          </div>
          <div className="metric-detail-grid">
            <Metric
              label="Usage"
              value={formatPercent(system?.cpu?.usagePercent)}
              detail={`${system?.cpu?.logicalCores ?? "—"} logical cores`}
            />
            <Metric
              label="Temperature"
              value={formatTemperature(system?.temperature?.celsius)}
              detail={system?.temperature?.source || "sensor unavailable"}
            />
            <Metric
              label="Load 1 min"
              value={system ? system.cpu.load1.toFixed(2) : "—"}
            />
            <Metric
              label="Load 5 / 15 min"
              value={
                system
                  ? `${system.cpu.load5.toFixed(2)} / ${system.cpu.load15.toFixed(2)}`
                  : "—"
              }
            />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">MEMORY</p>
              <h2>RAM & swap</h2>
            </div>
          </div>
          <div className="metric-detail-grid">
            <Metric
              label="RAM usage"
              value={formatPercent(system?.memory?.usagePercent)}
              detail={`${formatBytes(system?.memory?.usedBytes)} used`}
            />
            <Metric
              label="RAM available"
              value={formatBytes(system?.memory?.availableBytes)}
              detail={`${formatBytes(system?.memory?.totalBytes)} total`}
            />
            <Metric
              label="Swap usage"
              value={formatPercent(system?.memory?.swapUsagePercent)}
              detail={`${formatBytes(system?.memory?.swapUsedBytes)} used`}
            />
            <Metric
              label="Swap total"
              value={formatBytes(system?.memory?.swapTotalBytes)}
            />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">STORAGE</p>
              <h2>Root disk</h2>
            </div>
          </div>
          <div className="metric-detail-grid">
            <Metric
              label="Disk usage"
              value={formatPercent(system?.disk?.usagePercent)}
              detail={`${formatBytes(system?.disk?.usedBytes)} used`}
            />
            <Metric
              label="Available"
              value={formatBytes(system?.disk?.availableBytes)}
              detail={`${formatBytes(system?.disk?.totalBytes)} total`}
            />
            <Metric
              label="System uptime"
              value={formatUptime(system?.uptimeSeconds)}
            />
            <Metric
              label="Collection"
              value={loading ? "Loading" : "Live"}
              detail="5 second dashboard refresh"
            />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">NETWORK</p>
              <h2>{system?.network?.interface || "Network"}</h2>
            </div>
          </div>
          <div className="metric-detail-grid">
            <Metric
              label="Download"
              value={formatRate(system?.network?.rxBytesPerSecond)}
              detail={`${formatBytes(system?.network?.rxBytes)} received`}
            />
            <Metric
              label="Upload"
              value={formatRate(system?.network?.txBytesPerSecond)}
              detail={`${formatBytes(system?.network?.txBytes)} sent`}
            />
          </div>
        </article>
      </section>

      <section className="section-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">CONTROL PLANE</p>
            <h2>Service health</h2>
          </div>
        </div>

        <div className="service-list">
          {system?.services?.map((service) => (
            <div className="service-row" key={service.unit}>
              <div>
                <strong>{service.name}</strong>
                <small>{service.unit}</small>
              </div>
              <span className={service.active ? "service-up" : "service-down"}>
                {service.active ? "Active" : service.status}
              </span>
            </div>
          )) || (
            <div className="service-row">
              <strong>Services</strong>
              <span>{loading ? "Checking" : "Unavailable"}</span>
            </div>
          )}
        </div>
      </section>
    </>
  )
}
