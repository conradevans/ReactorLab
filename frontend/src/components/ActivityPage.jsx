import { useEffect, useRef, useState } from "react"

import {
  activityKindLabel,
  activitySeverityClass,
  activitySourceLabel,
  formatActivityTime,
} from "../activityMetrics"
import { formatDuration } from "../format"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
import usePollingJSON from "../usePollingJSON"

const recoveryEventIDPattern = /^[0-9a-f]{64}$/

function RecoveryIncidentDetails({ incident }) {
  if (!incident) return null

  return (
    <dl className="recovery-incident-details">
      <div>
        <dt>Last known alive</dt>
        <dd>{formatActivityTime(incident.lastKnownAliveAt)}</dd>
      </div>
      <div>
        <dt>Recovered</dt>
        <dd>{formatActivityTime(incident.recoveredAt)}</dd>
      </div>
      <div>
        <dt>Downtime</dt>
        <dd>{formatDuration(incident.downtimeSeconds)}</dd>
      </div>
      <div>
        <dt>Status</dt>
        <dd>{incident.status === "recovered" ? "Recovered" : "—"}</dd>
      </div>
    </dl>
  )
}

export default function ActivityPage({ eventID = "" }) {
  const selectedEventID = recoveryEventIDPattern.test(eventID) ? eventID : ""
  const requestPath = selectedEventID
    ? `/api/v1/activity?event=${encodeURIComponent(selectedEventID)}`
    : "/api/v1/activity"
  const { data, error, loading } = usePollingJSON(
    requestPath,
    ADMIN_POLL_INTERVAL_MS,
  )
  const events = Array.isArray(data?.events) ? data.events : []
  const selectedRowRef = useRef(null)
  const [highlightedEventID, setHighlightedEventID] = useState("")
  const selectedEventAvailable =
    selectedEventID !== "" &&
    events.some((event) => event.eventId === selectedEventID)

  useEffect(() => {
    if (!selectedEventAvailable) {
      return undefined
    }

    const row = selectedRowRef.current
    if (!row) return undefined

    row.scrollIntoView?.({ behavior: "smooth", block: "center" })
    row.focus({ preventScroll: true })
    setHighlightedEventID(selectedEventID)
    const timeout = window.setTimeout(() => {
      setHighlightedEventID((current) =>
        current === selectedEventID ? "" : current
      )
    }, 2400)
    return () => window.clearTimeout(timeout)
  }, [selectedEventAvailable, selectedEventID])

  const warningCount = events.filter(
    (event) => event.severity === "warning" || event.severity === "error",
  ).length
  const recoveryCount = events.filter(
    (event) =>
      event.kind === "recovery" ||
      event.kind === "unexpected_shutdown_recovery",
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
          <span className="activity-retention">
            7 day operational activity · recovery incidents retained
          </span>
        </div>

        <div className="activity-list">
          {events.map((event, index) => {
            const stableEventID = recoveryEventIDPattern.test(
              event.eventId || "",
            )
              ? event.eventId
              : ""
            const selected =
              selectedEventID !== "" && stableEventID === selectedEventID
            const highlighted = selected &&
              stableEventID === highlightedEventID

            return (
              <article
                className={
                  highlighted
                    ? "activity-row activity-row-selected"
                    : "activity-row"
                }
                id={stableEventID ? `activity-event-${stableEventID}` : undefined}
                key={
                  stableEventID ||
                  `${event.occurredAt}-${event.source}-${event.subject}-${index}`
                }
                ref={selected ? selectedRowRef : undefined}
                tabIndex={selected ? -1 : undefined}
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
                  {event.kind === "unexpected_shutdown_recovery" ? (
                    <RecoveryIncidentDetails incident={event.incident} />
                  ) : null}
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
            )
          })}

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
