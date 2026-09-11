import {
  formatPercent,
  formatTemperature,
  formatUptime,
} from "../format"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
import {
  batterySeverity,
  cpuLoadSeverity,
  diskSeverity,
  memorySeverity,
  temperatureSeverity,
} from "../systemStatus"
import usePollingJSON from "../usePollingJSON"
import InfoControl from "./InfoControl"

const metricInfo = {
  cpu: "Usage is the current share of CPU time in use. The status dot compares the 1- and 5-minute load averages with the Dell's logical-core capacity.",
  memory: "Linux memory usage is calculated from memory currently available to applications, including reclaimable cache.",
  disk: "The percentage of the Dell's root filesystem currently in use.",
  temperature: "The current CPU temperature and the Linux thermal sensor that reported it.",
  battery: "The Dell's current battery charge and whether Linux reports external AC power connected.",
}

function SeverityDot({ severity }) {
  if (!severity) return null
  const label = severity === "healthy"
    ? "Normal"
    : severity === "warning"
      ? "Warning"
      : "Critical"

  return (
    <span
      className={`metric-status-dot status-${severity}`}
      aria-label={`${label} status`}
      title={`${label} status`}
    />
  )
}

function OverviewMetric({ label, value, detail, explanation, severity }) {
  return (
    <article className="stat-card overview-stat-card">
      <div className="overview-stat-heading">
        <span>{label}</span>
        <InfoControl label={label}>{explanation}</InfoControl>
      </div>
      <SeverityDot severity={severity} />
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  )
}

function batteryPercent(battery) {
  return battery?.available && Number.isFinite(battery.percent)
    ? `${Math.round(battery.percent)}%`
    : "Unavailable"
}

function batteryDetail(battery) {
  if (!battery?.available) return "Battery hardware unavailable"
  if (!battery.acConnected) return "Running on battery"
  return battery.status ? `${battery.status} · AC power` : "AC power"
}

export default function OverviewPage() {
  const { data: system, error, loading } = usePollingJSON(
    "/api/v1/system",
    ADMIN_POLL_INTERVAL_MS,
  )

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
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 1s"}</span>
        </div>
      </section>

      <section className="stats-grid overview-primary-metrics">
        <OverviewMetric
          label="CPU"
          value={formatPercent(system?.cpu?.usagePercent)}
          detail={`${system?.cpu?.logicalCores ?? "—"} logical cores`}
          explanation={metricInfo.cpu}
          severity={cpuLoadSeverity(system?.cpu)}
        />
        <OverviewMetric
          label="Memory"
          value={formatPercent(system?.memory?.usagePercent)}
          detail="system RAM"
          explanation={metricInfo.memory}
          severity={memorySeverity(system?.memory?.usagePercent)}
        />
        <OverviewMetric
          label="Disk"
          value={formatPercent(system?.disk?.usagePercent)}
          detail="root filesystem"
          explanation={metricInfo.disk}
          severity={diskSeverity(system?.disk?.usagePercent)}
        />
        <OverviewMetric
          label="Temperature"
          value={formatTemperature(system?.temperature?.celsius)}
          detail={system?.temperature?.source || "CPU sensor"}
          explanation={metricInfo.temperature}
          severity={temperatureSeverity(system?.temperature?.celsius)}
        />
        <OverviewMetric
          label="Battery"
          value={batteryPercent(system?.battery)}
          detail={batteryDetail(system?.battery)}
          explanation={metricInfo.battery}
          severity={batterySeverity(system?.battery)}
        />
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
