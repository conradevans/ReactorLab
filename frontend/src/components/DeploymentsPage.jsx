import { formatBytes, formatPercent } from "../format"
import {
  deploymentStatusClass,
  deploymentStatusLabel,
  summarizeDeployment,
} from "../deploymentMetrics"
import {
  relationshipStateClass,
  relationshipStateLabel,
} from "../relationshipMetrics"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
import usePollingJSON from "../usePollingJSON"

function goToDeployment(navigate, app) {
  return (event) => {
    event.preventDefault()
    navigate(`/admin/deployments/${encodeURIComponent(app)}`)
  }
}

export default function DeploymentsPage({ navigate }) {
  const { data, error, loading } = usePollingJSON(
    "/api/v1/deployments",
    ADMIN_POLL_INTERVAL_MS,
  )
  const deployments = data?.deployments ?? []

  const healthy = deployments.filter(
    (deployment) => deployment.status === "healthy",
  ).length
  const containers = deployments.reduce(
    (count, deployment) => count + (deployment.containers?.length ?? 0),
    0,
  )
  const cpu = deployments.reduce(
    (total, deployment) => total + summarizeDeployment(deployment).cpuPercent,
    0,
  )
  const memory = deployments.reduce(
    (total, deployment) =>
      total + summarizeDeployment(deployment).memoryUsedBytes,
    0,
  )

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">MINIDEPLOY</p>
          <h1>Deployments</h1>
          <p>
            Live application and container resource usage from MiniDeploy's
            private control plane.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 1s"}</span>
        </div>
      </section>

      {error ? <div className="notice error">{error}</div> : null}

      <section className="deployment-summary-grid">
        <article className="stat-card">
          <span>Deployments</span>
          <strong>{loading ? "—" : deployments.length}</strong>
          <small>{healthy} healthy</small>
        </article>
        <article className="stat-card">
          <span>Containers</span>
          <strong>{loading ? "—" : containers}</strong>
          <small>Recorded by MiniDeploy</small>
        </article>
        <article className="stat-card">
          <span>CPU</span>
          <strong>{loading ? "—" : formatPercent(cpu)}</strong>
          <small>Combined live usage</small>
        </article>
        <article className="stat-card">
          <span>RAM</span>
          <strong>{loading ? "—" : formatBytes(memory)}</strong>
          <small>Combined container memory</small>
        </article>
      </section>

      <section className="deployment-list">
        {loading && deployments.length === 0 ? (
          <div className="empty-state resource-loading-state">
            <h3>Loading deployments…</h3>
            <p>Collecting MiniDeploy resources and database relationships.</p>
          </div>
        ) : null}

        {deployments.map((deployment) => {
          const summary = summarizeDeployment(deployment)
          const statusClass = deploymentStatusClass(deployment.status)

          return (
            <a
              className="deployment-row"
              href={`/admin/deployments/${encodeURIComponent(deployment.app)}`}
              key={deployment.app}
              onClick={goToDeployment(navigate, deployment.app)}
            >
              <div className="deployment-identity">
                <div>
                  <strong>{deployment.app}</strong>
                  <small>{deployment.strategy || "unknown strategy"}</small>
                  <small
                    className={`relationship-inline ${relationshipStateClass(
                      deployment.database?.state,
                    )}`}
                  >
                    Database ·{" "}
                    {deployment.database?.state === "linked"
                      ? deployment.database.displayName
                      : relationshipStateLabel(deployment.database?.state)}
                  </small>
                </div>
                <span className={`deployment-state ${statusClass}`}>
                  <span className="status-dot" aria-hidden="true" />
                  {deploymentStatusLabel(deployment.status)}
                </span>
              </div>

              <div className="deployment-row-metrics">
                <div>
                  <span>CPU</span>
                  <strong>{formatPercent(summary.cpuPercent)}</strong>
                </div>
                <div>
                  <span>RAM</span>
                  <strong>{formatBytes(summary.memoryUsedBytes)}</strong>
                </div>
                <div>
                  <span>Network</span>
                  <strong>
                    {formatBytes(
                      summary.networkRXBytes + summary.networkTXBytes,
                    )}
                  </strong>
                </div>
                <div>
                  <span>Containers</span>
                  <strong>{summary.containers}</strong>
                </div>
                <div>
                  <span>Restarts</span>
                  <strong>{summary.restarts}</strong>
                </div>
              </div>

              <span className="deployment-chevron" aria-hidden="true">→</span>
            </a>
          )
        })}

        {!loading && !error && deployments.length === 0 ? (
          <div className="empty-state">
            <h3>No deployments</h3>
            <p>MiniDeploy has no deployment records to monitor.</p>
          </div>
        ) : null}
      </section>
    </>
  )
}
