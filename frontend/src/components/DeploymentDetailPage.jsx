import { formatBytes, formatPercent, formatUptime } from "../format"
import {
  deploymentStatusClass,
  deploymentStatusLabel,
  summarizeDeployment,
} from "../deploymentMetrics"
import {
  relationshipStateClass,
  relationshipStateLabel,
} from "../relationshipMetrics"
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

function databaseRelationshipMessage(state) {
  if (state === "detached") return "This deployment has no MiniBase database attached."
  if (state === "unavailable") return "MiniBase relationship data is currently unavailable."
  if (state === "conflict") return "Multiple database relationships were detected for this deployment."
  return "Database relationship state is unavailable."
}

export default function DeploymentDetailPage({ app, navigate }) {
  const path = `/api/v1/deployments/${encodeURIComponent(app)}`
  const { data: deployment, error, loading } = usePollingJSON(path)
  const summary = summarizeDeployment(deployment)
  const databaseLink = deployment?.database

  function back(event) {
    event.preventDefault()
    navigate("/admin/deployments")
  }

  function openDatabase(event) {
    event.preventDefault()
    if (databaseLink?.state !== "linked" || !databaseLink.id) return
    navigate(`/admin/databases/${encodeURIComponent(databaseLink.id)}`)
  }

  return (
    <>
      <a className="back-link deployment-back" href="/admin/deployments" onClick={back}>
        ← Deployments
      </a>

      <section className="page-hero compact deployment-detail-hero">
        <div>
          <p className="eyebrow">MINIDEPLOY · DEPLOYMENT</p>
          <h1>{deployment?.app || app}</h1>
          <p>
            {deployment?.strategy || "Loading strategy"} · live resource usage
            for every recorded service container.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 5s"}</span>
        </div>
      </section>

      {error ? <div className="notice error">{error}</div> : null}

      <section className="deployment-summary-grid">
        <article className="stat-card">
          <span>Status</span>
          <strong className="deployment-status-text">
            {loading ? "—" : deploymentStatusLabel(deployment?.status)}
          </strong>
          <small>{deployment?.strategy || "—"}</small>
        </article>
        <article className="stat-card">
          <span>CPU</span>
          <strong>{loading ? "—" : formatPercent(summary.cpuPercent)}</strong>
          <small>Combined container usage</small>
        </article>
        <article className="stat-card">
          <span>RAM</span>
          <strong>{loading ? "—" : formatBytes(summary.memoryUsedBytes)}</strong>
          <small>
            {loading ? "—" : `${formatPercent(summary.memoryPercent)} of host`}
          </small>
        </article>
        <article className="stat-card">
          <span>Restarts</span>
          <strong>{loading ? "—" : summary.restarts}</strong>
          <small>{summary.containers} recorded containers</small>
        </article>
      </section>

      <section className="section-card relationship-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">MINIBASE</p>
            <h2>Database relationship</h2>
          </div>
          <span
            className={`relationship-badge ${relationshipStateClass(
              databaseLink?.state,
            )}`}
          >
            {loading ? "Loading" : relationshipStateLabel(databaseLink?.state)}
          </span>
        </div>

        {databaseLink?.state === "linked" ? (
          <a
            className="relationship-link"
            href={`/admin/databases/${encodeURIComponent(databaseLink.id)}`}
            onClick={openDatabase}
          >
            <div className="relationship-link-primary">
              <span>Database</span>
              <strong>{databaseLink.displayName}</strong>
              <small>{databaseLink.status || "unknown status"} · MiniBase</small>
            </div>
            <span className="relationship-arrow" aria-hidden="true">→</span>
          </a>
        ) : (
          <p className="relationship-empty">
            {loading
              ? "Loading database relationship…"
              : databaseRelationshipMessage(databaseLink?.state)}
          </p>
        )}
      </section>

      <section className="container-stack">
        {deployment?.containers?.map((container) => (
          <article className="section-card container-metrics-card" key={container.container}>
            <div className="section-heading container-heading">
              <div>
                <p className="eyebrow">{container.service.toUpperCase()}</p>
                <h2>{container.container}</h2>
                <small className="container-strategy">{container.strategy}</small>
              </div>
              <div className="container-state-stack">
                <span
                  className={`deployment-state ${deploymentStatusClass(
                    container.health === "healthy"
                      ? "healthy"
                      : container.state === "running"
                        ? "degraded"
                        : "unavailable",
                  )}`}
                >
                  <span className="status-dot" aria-hidden="true" />
                  {container.health === "healthy"
                    ? "Healthy"
                    : `${container.state} · ${container.health}`}
                </span>
              </div>
            </div>

            <div className="metric-detail-grid container-detail-grid">
              <Metric label="CPU" value={formatPercent(container.cpuPercent)} />
              <Metric
                label="RAM"
                value={formatBytes(container.memoryUsedBytes)}
                detail={`${formatPercent(container.memoryPercent)} of host`}
              />
              <Metric
                label="Network RX"
                value={formatBytes(container.networkRxBytes)}
              />
              <Metric
                label="Network TX"
                value={formatBytes(container.networkTxBytes)}
              />
              <Metric
                label="Block read"
                value={formatBytes(container.blockReadBytes)}
              />
              <Metric
                label="Block write"
                value={formatBytes(container.blockWriteBytes)}
              />
              <Metric
                label="Writable layer"
                value={formatBytes(container.writableBytes)}
              />
              <Metric label="PIDs" value={container.pids ?? "—"} />
              <Metric
                label="Uptime"
                value={formatUptime(container.uptimeSeconds)}
              />
              <Metric
                label="Restart count"
                value={container.restartCount ?? "—"}
              />
            </div>
          </article>
        ))}

        {!loading && !error && (deployment?.containers?.length ?? 0) === 0 ? (
          <div className="empty-state">
            <h3>No containers</h3>
            <p>This deployment has no recorded containers.</p>
          </div>
        ) : null}
      </section>
    </>
  )
}
