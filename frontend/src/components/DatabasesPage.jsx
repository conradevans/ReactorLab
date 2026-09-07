import { formatBytes, formatPercent } from "../format"
import {
  databaseStatusClass,
  databaseStatusLabel,
  formatBackupAge,
  summarizeDatabases,
} from "../databaseMetrics"
import {
  relationshipStateClass,
  relationshipStateLabel,
} from "../relationshipMetrics"
import usePollingJSON from "../usePollingJSON"

function goToDatabase(navigate, id) {
  return (event) => {
    event.preventDefault()
    navigate(`/admin/databases/${encodeURIComponent(id)}`)
  }
}

export default function DatabasesPage({ navigate }) {
  const { data, error, loading } = usePollingJSON("/api/v1/databases")
  const databases = data?.databases ?? []
  const postgres = data?.postgres
  const summary = summarizeDatabases(databases)
  const ready = databases.filter((database) => database.status === "ready").length

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">MINIBASE</p>
          <h1>Databases</h1>
          <p>
            Live database storage, connections, backups, and PostgreSQL service
            metrics from MiniBase&apos;s private control plane.
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
          <span>Databases</span>
          <strong>{loading ? "—" : databases.length}</strong>
          <small>{ready} ready</small>
        </article>
        <article className="stat-card">
          <span>Storage</span>
          <strong>{loading ? "—" : formatBytes(summary.sizeBytes)}</strong>
          <small>Managed database size</small>
        </article>
        <article className="stat-card">
          <span>Connections</span>
          <strong>{loading ? "—" : summary.connections}</strong>
          <small>Current PostgreSQL sessions</small>
        </article>
        <article className="stat-card">
          <span>Backups</span>
          <strong>{loading ? "—" : summary.backups}</strong>
          <small>{loading ? "—" : formatBytes(summary.backupBytes)} stored</small>
        </article>
      </section>

      <section className="section-card database-postgres-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">SHARED POSTGRESQL SERVICE</p>
            <h2>PostgreSQL</h2>
          </div>
          <span className={`deployment-state ${
            postgres?.state === "running" ? "deployment-healthy" : "deployment-error"
          }`}>
            <span className="status-dot" aria-hidden="true" />
            {postgres?.state || "unknown"}
          </span>
        </div>
        <div className="metric-detail-grid database-postgres-grid">
          <div className="metric-detail">
            <span>CPU</span>
            <strong>{loading ? "—" : formatPercent(postgres?.cpuPercent)}</strong>
          </div>
          <div className="metric-detail">
            <span>RAM</span>
            <strong>{loading ? "—" : formatBytes(postgres?.memoryUsedBytes)}</strong>
            <small>{loading ? "—" : `${formatPercent(postgres?.memoryPercent)} of host`}</small>
          </div>
          <div className="metric-detail">
            <span>Network RX</span>
            <strong>{loading ? "—" : formatBytes(postgres?.networkRxBytes)}</strong>
          </div>
          <div className="metric-detail">
            <span>Network TX</span>
            <strong>{loading ? "—" : formatBytes(postgres?.networkTxBytes)}</strong>
          </div>
          <div className="metric-detail">
            <span>Block read</span>
            <strong>{loading ? "—" : formatBytes(postgres?.blockReadBytes)}</strong>
          </div>
          <div className="metric-detail">
            <span>Block write</span>
            <strong>{loading ? "—" : formatBytes(postgres?.blockWriteBytes)}</strong>
          </div>
          <div className="metric-detail">
            <span>PIDs</span>
            <strong>{loading ? "—" : (postgres?.pids ?? "—")}</strong>
          </div>
        </div>
        <p className="database-shared-note">
          CPU, RAM, network, and block I/O belong to the shared PostgreSQL
          service and are not attributed to individual databases.
        </p>
      </section>

      <section className="database-list database-observability-list">
        {databases.map((database) => (
          <a
            className="database-row"
            href={`/admin/databases/${encodeURIComponent(database.id)}`}
            key={database.id}
            onClick={goToDatabase(navigate, database.id)}
          >
            <div className="database-primary">
              <strong>{database.displayName}</strong>
              <small>{formatBackupAge(database.backupAgeSeconds)}</small>
              <small
                className={`relationship-inline ${relationshipStateClass(
                  database.deployment?.state,
                )}`}
              >
                Deployment ·{" "}
                {database.deployment?.state === "linked"
                  ? database.deployment.app
                  : relationshipStateLabel(database.deployment?.state)}
              </small>
            </div>

            <span className={`status-badge ${databaseStatusClass(database.status)}`}>
              <span className="status-dot" aria-hidden="true" />
              {databaseStatusLabel(database.status).toUpperCase()}
            </span>

            <div className="database-meta">
              <span>Storage</span>
              <strong>{formatBytes(database.sizeBytes)}</strong>
            </div>

            <div className="database-meta">
              <span>Connections</span>
              <strong>{database.connections ?? "—"}</strong>
            </div>

            <span className="row-arrow" aria-hidden="true">→</span>
          </a>
        ))}

        {!loading && !error && databases.length === 0 ? (
          <div className="empty-state">
            <h3>No databases</h3>
            <p>MiniBase has no database records to monitor.</p>
          </div>
        ) : null}
      </section>
    </>
  )
}
