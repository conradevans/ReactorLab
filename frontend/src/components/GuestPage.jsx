import { formatUptime } from "../format"
import usePollingJSON from "../usePollingJSON"
import GlobalHeader from "./GlobalHeader"

export default function GuestPage({ navigate }) {
  const { data: status, error, loading } = usePollingJSON(
    "/api/v1/guest/status",
  )

  const health = loading
    ? "Checking"
    : status?.status === "ok"
      ? "Healthy"
      : "Unavailable"

  return (
    <main className="guest-page">
      <div className="site-shell">
        <GlobalHeader
          mode="guest"
          navigate={navigate}
          sessionLabel="Guest View"
        />

        <section className="guest-hero">
          <div>
            <p className="eyebrow">GUEST OVERVIEW</p>
            <h1>ReactorLab health.</h1>
            <p>
              A deliberately minimal public view. Detailed infrastructure
              information remains administrator-only.
            </p>
          </div>

          <div className="guest-summary">
            <div>
              <span>REACTORLAB</span>
              <strong>{health}</strong>
            </div>
            <div>
              <span>DELL UPTIME</span>
              <strong>{formatUptime(status?.uptimeSeconds)}</strong>
            </div>
          </div>
        </section>

        {error ? <div className="notice error">{error}</div> : null}

        <section className="content-section">
          <div className="section-card guest-boundary-card">
            <p className="eyebrow">PUBLIC BOUNDARY</p>
            <h2>Restricted by design</h2>
            <p className="placeholder-copy">
              Guest mode exposes only overall availability and uptime.
              Detailed host and application metrics require administrator
              access.
            </p>
          </div>
        </section>

        <footer>ReactorLab · Restricted guest dashboard</footer>
      </div>
    </main>
  )
}
