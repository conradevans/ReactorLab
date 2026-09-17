import { formatUptime } from "../format"
import usePollingJSON from "../usePollingJSON"
import GlobalHeader from "./GlobalHeader"

const emptySummary = { total: 0, shared: 0, hidden: 0 }

const databaseStatusLabels = {
  metadata_only: "Metadata only",
  provisioning: "Provisioning",
  creating: "Creating",
  ready: "Ready",
  error: "Error",
}

function ResourceSummary({ summary, title }) {
  const values = summary || emptySummary

  return (
    <div className="guest-resource-summary" aria-label={`${title} visibility summary`}>
      <div>
        <span>TOTAL</span>
        <strong>{values.total}</strong>
      </div>
      <div>
        <span>SHARED</span>
        <strong>{values.shared}</strong>
      </div>
      <div>
        <span>HIDDEN</span>
        <strong>{values.hidden}</strong>
      </div>
    </div>
  )
}

function DeploymentCard({ deployment }) {
  const running = deployment.status === "running"

  return (
    <article className="guest-resource-card guest-deployment-card">
      <div className="guest-deployment-primary">
        <h3 title={deployment.app}>{deployment.app}</h3>
        <span className={`status-pill ${running ? "live" : "down"}`}>
          <span
            className={`status-dot ${running ? "live" : "down"}`}
            aria-hidden="true"
          />
          {String(deployment.status || "unknown").toUpperCase()}
        </span>
      </div>

      <div className="guest-deployment-secondary">
        <a
          className="guest-deployment-url"
          href={deployment.url}
          target="_blank"
          rel="noreferrer"
          title={deployment.url}
        >
          {deployment.url}
        </a>
        <a
          className="guest-open-link"
          href={deployment.url}
          target="_blank"
          rel="noreferrer"
          aria-label={`Open ${deployment.app}`}
        >
          Open
          <span aria-hidden="true">↗</span>
        </a>
      </div>
    </article>
  )
}

function DatabaseCard({ database }) {
  const label = databaseStatusLabels[database.status] || database.status

  return (
    <article className="guest-resource-card guest-database-card">
      <h3>{database.displayName}</h3>
      <span className={`status-badge status-${database.status}`}>
        <span className="status-dot" aria-hidden="true" />
        {label}
      </span>
    </article>
  )
}

function ResourceSection({
  kind,
  title,
  eyebrow,
  section,
  loading,
  requestFailed,
}) {
  const available = !requestFailed && section?.available === true
  const items = available && Array.isArray(section.items) ? section.items : []
  const isDeployments = kind === "deployments"

  let content
  if (loading && !section) {
    content = (
      <div className="empty-state">
        Loading shared {isDeployments ? "deployments" : "databases"}…
      </div>
    )
  } else if (!available) {
    content = (
      <div className="notice error">
        {isDeployments
          ? "Deployment information is temporarily unavailable."
          : "Database information is temporarily unavailable."}
      </div>
    )
  } else if (items.length === 0) {
    content = (
      <div className="empty-state">
        No {isDeployments ? "deployments" : "databases"} are currently shared.
      </div>
    )
  } else {
    content = (
      <div className={`guest-resource-list ${kind}-list`}>
        {isDeployments
          ? items.map((deployment) => (
              <DeploymentCard deployment={deployment} key={deployment.app} />
            ))
          : items.map((database) => (
              <DatabaseCard database={database} key={database.id} />
            ))}
      </div>
    )
  }

  return (
    <section
      className="guest-resource-section"
      data-guest-resource-section={kind}
    >
      <div className="guest-resource-heading">
        <p className="eyebrow">{eyebrow}</p>
        <h2>{title}</h2>
      </div>
      <ResourceSummary
        summary={available ? section.summary : emptySummary}
        title={title}
      />
      {content}
    </section>
  )
}

export default function GuestPage({ navigate }) {
  const {
    data: resources,
    error: resourcesError,
    loading: resourcesLoading,
  } = usePollingJSON(
    "/api/v1/guest/resources",
  )
  const { data: status, error: statusError } = usePollingJSON(
    "/api/v1/guest/status",
  )
  const runtime = statusError
    ? "Unavailable"
    : formatUptime(status?.uptimeSeconds)

  return (
    <main className="guest-page">
      <div className="site-shell">
        <GlobalHeader
          mode="guest"
          navigate={navigate}
          sessionLabel="Guest View"
        />

        <section className="guest-hero reactor-guest-hero">
          <div className="reactor-guest-intro">
            <p className="eyebrow">REACTORLAB / GUEST</p>
            <h1>Personal developer cloud.</h1>
            <p>
              Applications and databases intentionally shared through
              MiniDeploy and MiniBase, presented through one read-only view.
            </p>
          </div>

          <div
            className="guest-summary guest-runtime-summary"
            aria-label="ReactorLab runtime summary"
          >
            <div>
              <span>TOTAL RUNTIME</span>
              <strong>{runtime}</strong>
            </div>
          </div>
        </section>

        <div className="guest-resource-columns" aria-label="Shared resources">
          <ResourceSection
            kind="deployments"
            title="Deployments"
            eyebrow="PUBLIC APPLICATIONS"
            section={resources?.deployments}
            loading={resourcesLoading}
            requestFailed={Boolean(resourcesError)}
          />
          <ResourceSection
            kind="databases"
            title="Databases"
            eyebrow="SAFE METADATA"
            section={resources?.databases}
            loading={resourcesLoading}
            requestFailed={Boolean(resourcesError)}
          />
        </div>

        <aside className="guest-boundary-note">
          <span aria-hidden="true">i</span>
          <p>
            Visibility is managed by MiniDeploy and MiniBase. Detailed host
            telemetry and administrative controls remain private.
          </p>
        </aside>

        <footer>ReactorLab · Shared resources · Read only</footer>
      </div>
    </main>
  )
}
