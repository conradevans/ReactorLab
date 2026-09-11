import {
  formatBytes,
  formatPercent,
  formatRate,
  formatTemperature,
  formatUptime,
} from "../format"
import { ADMIN_POLL_INTERVAL_MS } from "../polling"
import {
  batterySeverity,
  diskSeverity,
  memorySeverity,
  serviceSeverity,
  temperatureSeverity,
} from "../systemStatus"
import usePollingJSON from "../usePollingJSON"
import InfoControl from "./InfoControl"

const info = {
  cpuUsage: "The percentage of CPU time currently being used across the Dell's logical processors.",
  temperature: "The current CPU temperature reported by the system's thermal sensors.",
  load1: "The average number of runnable or waiting tasks during the last minute.",
  loadLong: "The average number of runnable or waiting tasks during the last 5 and 15 minutes.",
  memoryUsage: "The share of system memory currently in use, based on Linux available memory.",
  memoryAvailable: "System memory Linux currently reports as available for applications.",
  swapUsage: "The share of configured swap space currently in use.",
  swapTotal: "The total disk-backed swap space configured on the Dell.",
  diskUsage: "The percentage of the Dell's root filesystem currently used.",
  diskAvailable: "Storage currently available to non-privileged processes on the root filesystem.",
  uptime: "How long the Dell has been running since its last boot.",
  collection: "Whether this dashboard is receiving the latest live system sample.",
  download: "Current network receive activity on the default interface.",
  upload: "Current network transmit activity on the default interface.",
  battery: "The Dell's current battery charge. ReactorLab also checks whether external power is connected.",
  power: "Whether the Dell is currently connected to external AC power.",
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

function Metric({ label, value, detail, explanation, severity, valueClass = "" }) {
  return (
    <div className="metric-detail">
      <div className="metric-label">
        <span>{label}</span>
        <InfoControl label={label}>{explanation}</InfoControl>
      </div>
      <SeverityDot severity={severity} />
      <strong className={valueClass}>{value}</strong>
      {detail ? <small>{detail}</small> : null}
    </div>
  )
}

function batteryPercent(battery) {
  return battery?.available && Number.isFinite(battery.percent)
    ? `${Math.round(battery.percent)}%`
    : "Not detected"
}

export default function SystemPage() {
  const { data: system, error, loading } = usePollingJSON(
    "/api/v1/system",
    ADMIN_POLL_INTERVAL_MS,
  )
  const battery = system?.battery
  const powerSeverity = batterySeverity(battery)
  const powerLabel = !battery?.available
    ? "Unavailable"
    : battery.acConnected
      ? "Connected"
      : "Running on battery"

  return (
    <>
      <section className="page-hero">
        <div>
          <p className="eyebrow">SYSTEM</p>
          <h1>Dell system</h1>
          <p>
            Live host resource usage, networking, uptime, power, and ReactorLab
            service health.
          </p>
        </div>
        <div className="live-refresh">
          <span className="status-dot status-ready" aria-hidden="true" />
          <span>{error ? "Metrics unavailable" : "Live · refreshes every 1s"}</span>
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
              explanation={info.cpuUsage}
            />
            <Metric
              label="Temperature"
              value={formatTemperature(system?.temperature?.celsius)}
              detail={system?.temperature?.source || "sensor unavailable"}
              explanation={info.temperature}
              severity={temperatureSeverity(system?.temperature?.celsius)}
            />
            <Metric
              label="Load 1 min"
              value={system ? system.cpu.load1.toFixed(2) : "—"}
              explanation={info.load1}
            />
            <Metric
              label="Load 5 / 15 min"
              value={
                system
                  ? `${system.cpu.load5.toFixed(2)} / ${system.cpu.load15.toFixed(2)}`
                  : "—"
              }
              explanation={info.loadLong}
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
              explanation={info.memoryUsage}
              severity={memorySeverity(system?.memory?.usagePercent)}
            />
            <Metric
              label="RAM available"
              value={formatBytes(system?.memory?.availableBytes)}
              detail={`${formatBytes(system?.memory?.totalBytes)} total`}
              explanation={info.memoryAvailable}
            />
            <Metric
              label="Swap usage"
              value={formatPercent(system?.memory?.swapUsagePercent)}
              detail={`${formatBytes(system?.memory?.swapUsedBytes)} used`}
              explanation={info.swapUsage}
            />
            <Metric
              label="Swap total"
              value={formatBytes(system?.memory?.swapTotalBytes)}
              explanation={info.swapTotal}
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
              explanation={info.diskUsage}
              severity={diskSeverity(system?.disk?.usagePercent)}
            />
            <Metric
              label="Available"
              value={formatBytes(system?.disk?.availableBytes)}
              detail={`${formatBytes(system?.disk?.totalBytes)} total`}
              explanation={info.diskAvailable}
            />
            <Metric
              label="System uptime"
              value={formatUptime(system?.uptimeSeconds)}
              explanation={info.uptime}
            />
            <Metric
              label="Collection"
              value={loading ? "Loading" : "Live"}
              detail="About one second between completed samples"
              explanation={info.collection}
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
              explanation={info.download}
            />
            <Metric
              label="Upload"
              value={formatRate(system?.network?.txBytesPerSecond)}
              detail={`${formatBytes(system?.network?.txBytes)} sent`}
              explanation={info.upload}
            />
          </div>
        </article>

        <article className="section-card battery-card">
          <div className="section-heading">
            <div>
              <p className="eyebrow">POWER</p>
              <h2>Battery</h2>
            </div>
          </div>
          <div className="metric-detail-grid">
            <Metric
              label="Charge"
              value={batteryPercent(battery)}
              detail={battery?.available ? battery.status || "status unavailable" : "No battery reported"}
              explanation={info.battery}
            />
            <Metric
              label="External power"
              value={powerLabel}
              explanation={info.power}
              severity={powerSeverity}
              valueClass={powerSeverity === "critical" ? "metric-critical-value" : ""}
            />
          </div>
        </article>
      </section>

      <section className="section-card service-health-card">
        <div className="section-heading">
          <div>
            <p className="eyebrow">CONTROL PLANE</p>
            <h2>Service health</h2>
          </div>
          <InfoControl label="Service health">
            Whether the ReactorLab platform services are currently active.
          </InfoControl>
        </div>

        <div className="service-list">
          {system?.services?.map((service) => (
            <div className="service-row" key={service.unit}>
              <div>
                <strong>{service.name}</strong>
                <small>{service.unit}</small>
              </div>
              <div className="service-status-wrap">
                <InfoControl label={`${service.name} service`}>
                  Whether the {service.name} service is currently active.
                </InfoControl>
                <SeverityDot severity={serviceSeverity(service)} />
                <span className={service.active ? "service-up" : "service-down"}>
                  {service.active ? "Active" : service.status}
                </span>
              </div>
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
