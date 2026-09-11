import { formatBytes, formatPercent } from "../format"
import {
  cacheHitPercent,
  databaseStatusClass,
  databaseStatusLabel,
  formatBackupAge,
} from "../databaseMetrics"
import {
  relationshipStateClass,
  relationshipStateLabel,
} from "../relationshipMetrics"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
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

function formatTimestamp(value) {
  if (!value) return "No completed backup"

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "Unknown"

  return date.toLocaleString()
}

function deploymentRelationshipMessage(link) {
  if (link?.state === "detached") return "This database is not attached to a MiniDeploy deployment."
  if (link?.state === "unavailable") return "MiniDeploy relationship data is currently unavailable."
  if (link?.state === "unresolved") {
    return link.app
      ? `${link.app} is attached in MiniBase, but MiniDeploy did not return that deployment.`
      : "The attached MiniDeploy deployment could not be resolved."
  }
  if (link?.state === "conflict") return "Multiple deployment relationships were detected for this database."
  return "Deployment relationship state is unavailable."
}

export default function DatabaseDetailPage({ id, navigate }) {
  const path = `/api/v1/databases/${encodeURIComponent(id)}`
  const { data: database, error, loading } = usePollingJSON(
    path,
    ADMIN_POLL_INTERVAL_MS,
  )
  const cachePercent = cacheHitPercent(database)
  const deploymentLink = database?.deployment

  function back(event) {
    event.preventDefault()
    navigate("/admin/databases")
  }

  function openDeployment(event) {
    event.preventDefault()
    if (deploymentLink?.state !== "linked" || !deploymentLink.app) return
    navigate(`/admin/deployments/${encodeURIComponent(deploymentLink.app)}`)
  }

  return (
    <>
      <a className="back-link database-observability-back" href="/admin/databases" onClick={back}>
        ← Databases
      </a>

      <section className="page-hero compact database-detail-hero">
        <div>
          <p className="eyebrow">MINIBASE · DATABASE</p>
          <h1>{database?.displayName || "Database"}</h1>
          <p>
            Live PostgreSQL activity and backup metrics for this MiniBase
            database.
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
          <span>Status</span>
          <strong>{loading ? "—" : databaseStatusLabel(database?.status)}</strong>
          <small>
            {loading ? "—" : (
              <span className={`status-badge ${databaseStatusClass(database?.status)}`}>
                <span className="status-dot" aria-hidden="true" />
                {databaseStatusLabel(database?.status).toUpperCase()}
              </span>
            )}
          </small>
        </article>

        <article className="stat-card">
          <span>Storage</span>
          <strong>{loading ? "—" : formatBytes(database?.sizeBytes)}</strong>
          <small>Current database size</small>
        </article>

        <article className="stat-card">
          <span>Connections</span>
          <strong>{loading ? "—" : (database?.connections ?? "—")}</strong>
          <small>Current PostgreSQL sessions</small>
        </article>

        <article className="stat-card">
          <span>Backups</span>
          <strong>{loading ? "—" : (database?.backupCount ?? "—")}</strong>
          <small>{loading ? "—" : `${formatBytes(database?.backupBytes)} stored`}</small>
        </article>
      </section>

      <section className="section-card relationship-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">MINIDEPLOY</p>
            <h2>Deployment relationship</h2>
          </div>
          <span
            className={`relationship-badge ${relationshipStateClass(
              deploymentLink?.state,
            )}`}
          >
            {loading ? "Loading" : relationshipStateLabel(deploymentLink?.state)}
          </span>
        </div>

        {deploymentLink?.state === "linked" ? (
          <a
            className="relationship-link"
            href={`/admin/deployments/${encodeURIComponent(deploymentLink.app)}`}
            onClick={openDeployment}
          >
            <div className="relationship-link-primary">
              <span>Deployment</span>
              <strong>{deploymentLink.app}</strong>
              <small>
                {deploymentLink.strategy || "unknown strategy"} ·{" "}
                {deploymentLink.status || "unknown status"}
              </small>
            </div>
            <span className="relationship-arrow" aria-hidden="true">→</span>
          </a>
        ) : (
          <p className="relationship-empty">
            {loading
              ? "Loading deployment relationship…"
              : deploymentRelationshipMessage(deploymentLink)}
          </p>
        )}
      </section>

      <section className="system-section-grid database-section-grid">
        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">CONNECTIONS</p>
              <h2>Connection activity</h2>
            </div>
          </div>
          <div className="metric-detail-grid database-detail-grid">
            <Metric label="Total" value={database?.connections ?? "—"} />
            <Metric label="Active" value={database?.activeConnections ?? "—"} />
            <Metric label="Idle" value={database?.idleConnections ?? "—"} />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">TRANSACTIONS & CACHE</p>
              <h2>Database activity</h2>
            </div>
          </div>
          <div className="metric-detail-grid database-detail-grid">
            <Metric label="Commits" value={database?.transactions?.commits ?? "—"} />
            <Metric label="Rollbacks" value={database?.transactions?.rollbacks ?? "—"} />
            <Metric label="Block reads" value={database?.cache?.blockReads ?? "—"} />
            <Metric
              label="Cache hit rate"
              value={cachePercent == null ? "—" : formatPercent(cachePercent)}
              detail={
                database
                  ? `${database.cache?.blockHits ?? 0} cache hits`
                  : undefined
              }
            />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">ROWS</p>
              <h2>Row changes</h2>
            </div>
          </div>
          <div className="metric-detail-grid database-detail-grid">
            <Metric label="Inserted" value={database?.rows?.inserted ?? "—"} />
            <Metric label="Updated" value={database?.rows?.updated ?? "—"} />
            <Metric label="Deleted" value={database?.rows?.deleted ?? "—"} />
          </div>
        </article>

        <article className="section-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">BACKUPS</p>
              <h2>Backup health</h2>
            </div>
          </div>
          <div className="metric-detail-grid database-detail-grid">
            <Metric label="Backup count" value={database?.backupCount ?? "—"} />
            <Metric label="Stored" value={formatBytes(database?.backupBytes)} />
            <Metric
              label="Latest backup"
              value={formatBackupAge(database?.backupAgeSeconds)}
              detail={formatTimestamp(database?.latestBackupAt)}
            />
          </div>
        </article>
      </section>

      <p className="database-shared-note database-detail-note">
        CPU, RAM, network, and block I/O are shared by the PostgreSQL service
        and are shown on the Databases overview instead of being attributed to
        this database.
      </p>
    </>
  )
}
