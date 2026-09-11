import {
  activityKindLabel,
  activitySeverityClass,
  activitySourceLabel,
  formatActivityTime,
} from "../activityMetrics"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
import usePollingJSON from "../usePollingJSON"

export default function ActivityPage() {
  const { data, error, loading } = usePollingJSON(
    "/api/v1/activity",
    ADMIN_POLL_INTERVAL_MS,
  )
  const events = data?.events || []

  const warningCount = events.filter(
    (event) => event.severity === "warning" || event.severity === "error",
  ).length
  const recoveryCount = events.filter(
    (event) => event.kind === "recovery",
  ).length
  const sourceCount = new Set(events.map((event) => event.source)).size

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">ACTIVITY</p>
          <h1>Monitoring activity</h1>
          <p>
            Persisted warnings and recovery events from the Dell, MiniDeploy,
            MiniBase, and ReactorLab relationships.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Activity unavailable" : "Live · refreshes every 1s"}</span>
        </div>
      </section>

      {error ? <div className="notice error">{error}</div> : null}

      <section className="deployment-summary-grid">
        <article className="stat-card">
          <span>Recent events</span>
          <strong>{loading ? "—" : events.length}</strong>
          <small>Retained monitoring activity</small>
        </article>
        <article className="stat-card">
          <span>Warnings</span>
          <strong>{loading ? "—" : warningCount}</strong>
          <small>Warning or error events</small>
        </article>
        <article className="stat-card">
          <span>Recoveries</span>
          <strong>{loading ? "—" : recoveryCount}</strong>
          <small>Conditions returned to normal</small>
        </article>
        <article className="stat-card">
          <span>Sources</span>
          <strong>{loading ? "—" : sourceCount}</strong>
          <small>Reporting in this activity window</small>
        </article>
      </section>

      <section className="section-card activity-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">TIMELINE</p>
            <h2>Recent events</h2>
          </div>
          <span className="activity-retention">7 day retention</span>
        </div>

        <div className="activity-list">
          {events.map((event, index) => (
            <article
              className="activity-row"
              key={`${event.occurredAt}-${event.source}-${event.subject}-${index}`}
            >
              <div
                className={`activity-marker ${activitySeverityClass(
                  event.severity,
                )}`}
                aria-hidden="true"
              />
              <div className="activity-content">
                <div className="activity-row-heading">
                  <div>
                    <span className="activity-source">
                      {activitySourceLabel(event.source)}
                    </span>
                    <strong>{event.subject || "Monitoring event"}</strong>
                  </div>
                  <time dateTime={event.occurredAt || undefined}>
                    {formatActivityTime(event.occurredAt)}
                  </time>
                </div>
                <p>{event.message}</p>
                <div className="activity-meta">
                  <span
                    className={`activity-kind ${activitySeverityClass(
                      event.severity,
                    )}`}
                  >
                    {activityKindLabel(event.kind)}
                  </span>
                  <span>{event.severity || "info"}</span>
                </div>
              </div>
            </article>
          ))}

          {!loading && events.length === 0 ? (
            <div className="activity-empty">
              <strong>No monitoring events yet</strong>
              <p>
                Warnings and recoveries will appear here when ReactorLab detects
                a meaningful state change.
              </p>
            </div>
          ) : null}

          {loading && events.length === 0 ? (
            <div className="activity-empty">
              <strong>Loading activity…</strong>
            </div>
          ) : null}
        </div>
      </section>
    </>
  )
}
