import { useEffect, useMemo, useRef, useState } from "react"

import {
  CHART_GEOMETRY,
  formatInspectionTimestamp,
  inspectChartPosition,
  interpolateChartValue,
  prepareChartPoints,
  summarizeChartSeries,
} from "../chartInspection"
import { formatBytes, formatPercent, formatRate, formatTemperature } from "../format"
import {
  getApplicationObservability,
  getObservabilityOverview,
  OBSERVABILITY_RANGES,
} from "../observabilityApi"

import "./ObservabilityPage.css"

const RANGE_LABELS = {
  "15m": "15M",
  "1h": "1H",
  "6h": "6H",
  "24h": "24H",
  "7d": "7D",
}

const COLORS = ["#79a8ff", "#60d394", "#f2bd5d", "#c09cff"]

function finite(value) {
  return Number.isFinite(value) ? value : null
}

function percent(part, total) {
  return Number.isFinite(part) && Number.isFinite(total) && total > 0
    ? (part / total) * 100
    : null
}

function timeLabel(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

function eventTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"
  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function eventLabel(value) {
  return String(value || "Infrastructure event")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (letter) => letter.toUpperCase())
}

export function Chart({ title, description, points, series, formatValue, fixedMax }) {
  const svgRef = useRef(null)
  const [inspection, setInspection] = useState(null)
  const chart = useMemo(() => {
    const { width, height, top, bottom, left, right } = CHART_GEOMETRY
    const orderedPoints = prepareChartPoints(points)
    const values = []
    for (const entry of orderedPoints) {
      for (const item of series) {
        const value = finite(item.value(entry.point))
        if (value !== null) values.push(value)
      }
    }
    if (values.length === 0) return null
    const maximum = fixedMax || Math.max(...values, 1)
    const minimum = Math.min(0, ...values)
    const range = Math.max(maximum - minimum, 1)
    const start = orderedPoints[0].timestamp
    const end = orderedPoints.at(-1).timestamp
    const duration = Math.max(end - start, 1)
    const x = (entry) => left + ((entry.timestamp - start) / duration) * (width - left - right)
    const y = (value) => top + ((maximum - value) / range) * (height - top - bottom)
    const lines = series.map((item) => {
      let path = ""
      let drawing = false
      for (const entry of orderedPoints) {
        const value = finite(item.value(entry.point))
        if (value === null) {
          drawing = false
          continue
        }
        path += `${drawing ? " L" : "M"} ${x(entry).toFixed(1)} ${y(value).toFixed(1)}`
        drawing = true
      }
      return { ...item, path }
    })
    return {
      bottom,
      end,
      height,
      left,
      lines,
      orderedPoints,
      right,
      start,
      top,
      width,
    }
  }, [fixedMax, points, series])

  const summaryValues = useMemo(
    () => summarizeChartSeries(points, series),
    [points, series],
  )

  function updateInspection(event, pinned) {
    if (!chart || !svgRef.current) return
    const position = inspectChartPosition(
      event.clientX,
      svgRef.current.getBoundingClientRect(),
      chart.start,
      chart.end,
    )
    setInspection({ ...position, pinned, points })
  }

  function handlePointerMove(event) {
    if ((event.pointerType || "mouse") !== "mouse") return
    updateInspection(event, false)
  }

  function handlePointerDown(event) {
    if ((event.pointerType || "mouse") === "mouse") return
    updateInspection(event, true)
  }

  const activeInspection = inspection?.points === points ? inspection : null
  const inspectionValues = chart && activeInspection
    ? series.flatMap((item, index) => (item.showInTooltip ? [{
      color: item.color || COLORS[index],
      label: item.tooltipLabel || item.label,
      value: interpolateChartValue(
        chart.orderedPoints,
        item.value,
        activeInspection.timestamp,
      ),
    }] : []))
    : []

  return (
    <article className="observability-chart-card">
      <div className="chart-heading">
        <div>
          <h3>{title}</h3>
          <p>{description}</p>
        </div>
        <div className="chart-legend">
          {series.map((item, index) => (
            <span key={item.label}>
              <i style={{ background: item.color || COLORS[index] }} />
              {item.label}
            </span>
          ))}
        </div>
      </div>
      {!chart ? (
        <div className="chart-empty">No samples in this range</div>
      ) : (
        <div className="chart-wrap">
          <div className="chart-canvas">
            <svg
              aria-label={`${title} historical chart`}
              className="history-chart"
              ref={svgRef}
              role="img"
              viewBox={`0 0 ${chart.width} ${chart.height}`}
            >
              <line x1={chart.left} x2={chart.width - chart.right} y1={chart.top} y2={chart.top} />
              <line x1={chart.left} x2={chart.width - chart.right} y1="89" y2="89" />
              <line x1={chart.left} x2={chart.width - chart.right} y1={chart.height - chart.bottom} y2={chart.height - chart.bottom} />
              {chart.lines.map((line, index) => (
                <path
                  className={line.dashed ? "chart-line dashed" : "chart-line"}
                  d={line.path}
                  key={line.label}
                  stroke={line.color || COLORS[index]}
                />
              ))}
              <rect
                aria-hidden="true"
                className="chart-inspection-target"
                data-testid="chart-inspection-target"
                height={chart.height - chart.top - chart.bottom}
                onPointerDown={handlePointerDown}
                onPointerEnter={handlePointerMove}
                onPointerLeave={() => {
                  setInspection((current) => (
                    current?.points === points && current.pinned ? current : null
                  ))
                }}
                onPointerMove={handlePointerMove}
                width={chart.width - chart.left - chart.right}
                x={chart.left}
                y={chart.top}
              />
              {activeInspection ? (
                <line
                  aria-hidden="true"
                  className="chart-crosshair"
                  data-testid="chart-crosshair"
                  x1={activeInspection.x}
                  x2={activeInspection.x}
                  y1={chart.top}
                  y2={chart.height - chart.bottom}
                />
              ) : null}
            </svg>
            {activeInspection ? (
              <div
                className={`chart-inspection-tooltip ${activeInspection.normalized > 0.62 ? "align-left" : "align-right"}`}
                data-testid="chart-inspection-tooltip"
                style={{ "--inspection-left": `${(activeInspection.x / chart.width) * 100}%` }}
              >
                <time
                  dateTime={new Date(activeInspection.timestamp).toISOString()}
                  data-testid="chart-inspection-time"
                >
                  {formatInspectionTimestamp(activeInspection.timestamp)}
                </time>
                <dl>
                  {inspectionValues.map((item) => (
                    <div key={item.label}>
                      <dt>
                        <i style={{ background: item.color }} />
                        {item.label}
                      </dt>
                      <dd style={{ color: item.color }}>
                        {item.value === null ? "—" : formatValue(item.value)}
                      </dd>
                    </div>
                  ))}
                </dl>
              </div>
            ) : null}
          </div>
          <div className="chart-axis">
            <span>{timeLabel(chart.start)}</span>
            <span>{timeLabel(chart.end)}</span>
          </div>
          <div className="chart-latest">
            {summaryValues.map((item, index) => (
              <span key={item.label} style={{ color: item.color || COLORS[index] }}>
                {item.label} {item.summaryValue === null ? "—" : formatValue(item.summaryValue)}
              </span>
            ))}
          </div>
        </div>
      )}
    </article>
  )
}

function ServiceHistory({ service }) {
  const latest = service.points.at(-1)
  return (
    <article className="service-history-card">
      <div>
        <strong>{service.name}</strong>
        <span className={latest?.available ? "health-label healthy" : "health-label unavailable"}>
          {latest ? (latest.available ? "Healthy" : "Unavailable") : "No data"}
        </span>
      </div>
      <div className="health-strip" aria-label={`${service.name} availability history`}>
        {service.points.length === 0 ? <span className="health-segment unknown" /> : null}
        {service.points.map((point, index) => (
          <span
            className={point.available ? "health-segment healthy" : "health-segment unavailable"}
            key={`${point.timestamp}-${index}`}
            title={`${eventTime(point.timestamp)} · ${point.status}`}
          />
        ))}
      </div>
    </article>
  )
}

export default function ObservabilityPage() {
  const [range, setRange] = useState("1h")
  const [overview, setOverview] = useState(null)
  const [selectedApp, setSelectedApp] = useState("")
  const [appPoints, setAppPoints] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [appError, setAppError] = useState("")
  const [refreshVersion, setRefreshVersion] = useState(0)
  const selectedAppRef = useRef("")

  useEffect(() => {
    let active = true
    let refreshTimer
    const controller = new AbortController()

    async function refresh() {
      try {
        const next = await getObservabilityOverview(range, {
          signal: controller.signal,
        })
        if (!active) return

        setOverview(next)
        setError("")

        const previousApp = selectedAppRef.current
        const appID = next.applications.some((app) => app.id === previousApp)
          ? previousApp
          : next.applications[0]?.id || ""
        if (appID !== previousApp) {
          setAppPoints([])
          setAppError("")
        }
        selectedAppRef.current = appID
        setSelectedApp(appID)

        if (!appID) {
          setAppPoints([])
          setAppError("")
          return
        }

        try {
          const points = await getApplicationObservability(appID, range, {
            signal: controller.signal,
          })
          if (!active || selectedAppRef.current !== appID) return
          setAppPoints(points)
          setAppError("")
        } catch (loadError) {
          if (
            !active ||
            loadError?.name === "AbortError" ||
            selectedAppRef.current !== appID
          ) return
          setAppPoints([])
          setAppError("Application history is unavailable.")
        }
      } catch (loadError) {
        if (!active || loadError?.name === "AbortError") return
        setError("Historical observability is unavailable.")
      } finally {
        if (active) {
          setLoading(false)
          refreshTimer = window.setTimeout(refresh, 30_000)
        }
      }
    }

    void refresh()
    return () => {
      active = false
      controller.abort()
      window.clearTimeout(refreshTimer)
    }
  }, [range, refreshVersion])

  const host = overview?.host || []
  const temperature = overview?.temperature || []
  const applications = overview?.applications || []
  const services = overview?.services || []
  const events = overview?.events || []
  const selectedSummary = applications.find((app) => app.id === selectedApp)

  return (
    <>
      <section className="page-hero observability-hero">
        <div>
          <p className="eyebrow">HISTORICAL OBSERVABILITY</p>
          <h1>Infrastructure over time</h1>
          <p>
            Seven days of host, application, and platform health correlated with
            durable infrastructure events.
          </p>
        </div>
        <div className="range-controls" aria-label="Observability time range">
          {OBSERVABILITY_RANGES.map((item) => (
            <button
              aria-pressed={range === item}
              className={range === item ? "range-button active" : "range-button"}
              key={item}
              onClick={() => {
                if (item === range) return
                setLoading(true)
                setOverview(null)
                setAppPoints([])
                setError("")
                setAppError("")
                setRange(item)
              }}
              type="button"
            >
              {RANGE_LABELS[item]}
            </button>
          ))}
        </div>
      </section>

      {error ? <div className="notice error">{error}</div> : null}
      {loading && !overview ? <div className="observability-loading">Loading historical samples…</div> : null}

      <section className="observability-section" aria-labelledby="host-history-heading">
        <div className="section-heading">
          <div>
            <p className="eyebrow">HOST</p>
            <h2 id="host-history-heading">Dell history</h2>
          </div>
          <span className="history-window">{RANGE_LABELS[range]} ending now</span>
        </div>
        <div className="observability-chart-grid">
          <Chart title="CPU" description="Average with preserved maximum" points={host} fixedMax={100} formatValue={formatPercent} series={[
            { label: "Average", tooltipLabel: "CPU", value: (point) => point.cpuAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "Maximum", value: (point) => point.cpuMaximum, summaryAggregation: "max", showInTooltip: false, color: COLORS[2], dashed: true },
          ]} />
          <Chart title="Memory" description="Used memory percentage" points={host} fixedMax={100} formatValue={formatPercent} series={[
            { label: "Average", tooltipLabel: "Memory", value: (point) => percent(point.memoryUsedAverage, point.memoryTotal), summaryAggregation: "average", showInTooltip: true },
            { label: "Maximum", value: (point) => percent(point.memoryUsedMaximum, point.memoryTotal), summaryAggregation: "max", showInTooltip: false, color: COLORS[2], dashed: true },
          ]} />
          <Chart title="Temperature" description="Average and peak preserve brief thermal spikes" points={temperature} formatValue={formatTemperature} series={[
            { label: "Average", tooltipLabel: "Temperature", value: (point) => point.avgCelsius, summaryAggregation: "average", showInTooltip: true },
            { label: "Peak", value: (point) => point.maxCelsius, summaryAggregation: "max", showInTooltip: false, color: "#fb7185", dashed: true },
          ]} />
          <Chart title="Load" description="Linux load averages" points={host} formatValue={(value) => value.toFixed(2)} series={[
            { label: "1m", value: (point) => point.load1Average, summaryAggregation: "average", showInTooltip: true },
            { label: "5m", value: (point) => point.load5Average, summaryAggregation: "average", showInTooltip: true, color: COLORS[1] },
            { label: "15m", value: (point) => point.load15Average, summaryAggregation: "average", showInTooltip: true, color: COLORS[2] },
          ]} />
          <Chart title="Disk capacity" description="Root filesystem used" points={host} fixedMax={100} formatValue={formatPercent} series={[
            { label: "Used", value: (point) => percent(point.diskUsedAverage, point.diskTotal), summaryAggregation: "average", showInTooltip: true },
          ]} />
          <Chart title="Disk I/O" description="Root backing device throughput" points={host} formatValue={formatRate} series={[
            { label: "Read", value: (point) => point.diskReadAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "Write", value: (point) => point.diskWriteAverage, summaryAggregation: "average", showInTooltip: true, color: COLORS[1] },
          ]} />
          <Chart title="Network" description="Default-route interface throughput" points={host} formatValue={formatRate} series={[
            { label: "RX", value: (point) => point.networkRxAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "TX", value: (point) => point.networkTxAverage, summaryAggregation: "average", showInTooltip: true, color: COLORS[1] },
          ]} />
        </div>
      </section>

      <section className="observability-section" aria-labelledby="application-history-heading">
        <div className="section-heading app-history-heading">
          <div>
            <p className="eyebrow">APPLICATIONS</p>
            <h2 id="application-history-heading">Per-application history</h2>
          </div>
          <label className="app-selector">
            <span>Application</span>
            <select
              value={selectedApp}
              onChange={(event) => {
                const appID = event.target.value
                selectedAppRef.current = appID
                setAppPoints([])
                setAppError("")
                setSelectedApp(appID)
                setRefreshVersion((version) => version + 1)
              }}
            >
              {applications.length === 0 ? <option value="">No applications</option> : null}
              {applications.map((app) => <option key={app.id} value={app.id}>{app.name}</option>)}
            </select>
          </label>
        </div>
        {appError ? <div className="notice error">{appError}</div> : null}
        {selectedSummary ? (
          <div className="application-history-summary">
            <div><span>Application</span><strong>{selectedSummary.name}</strong></div>
            <div><span>Latest status</span><strong>{selectedSummary.latestStatus}</strong></div>
            <div><span>Restart count</span><strong>{selectedSummary.restartCount}</strong></div>
            <div><span>Last observed</span><strong>{eventTime(selectedSummary.lastObservedAt)}</strong></div>
          </div>
        ) : null}
        {appPoints.length > 0 ? (
          <article className="application-state-history">
            <div className="application-state-heading">
              <div>
                <strong>Status history</strong>
                <span>Unavailable samples remain visible after downsampling</span>
              </div>
              <span>
                Restarts {appPoints[0].restartCount} → {appPoints.at(-1).restartCount}
              </span>
            </div>
            <div className="health-strip" aria-label="Application status and restart history">
              {appPoints.map((point, index) => (
                <span
                  className={point.status === "healthy" ? "health-segment healthy" : "health-segment unavailable"}
                  key={`${point.timestamp}-${index}`}
                  title={`${eventTime(point.timestamp)} · ${point.status} · ${point.restartCount} restarts`}
                />
              ))}
            </div>
          </article>
        ) : null}
        <div className="observability-chart-grid application-charts">
          <Chart title="Application CPU" description="Container CPU for the selected application" points={appPoints} formatValue={formatPercent} series={[
            { label: "Average", tooltipLabel: "CPU", value: (point) => point.cpuAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "Maximum", value: (point) => point.cpuMaximum, summaryAggregation: "max", showInTooltip: false, color: COLORS[2], dashed: true },
          ]} />
          <Chart title="Application memory" description="Used memory across application containers" points={appPoints} formatValue={formatBytes} series={[
            { label: "Used", value: (point) => point.memoryUsedAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "Maximum", value: (point) => point.memoryUsedMaximum, summaryAggregation: "max", showInTooltip: false, color: COLORS[2], dashed: true },
            { label: "Limit", value: (point) => point.memoryLimitAverage, summaryAggregation: "average", showInTooltip: true, color: COLORS[3], dashed: true },
          ]} />
          <Chart title="Application network" description="Container receive and transmit throughput" points={appPoints} formatValue={formatRate} series={[
            { label: "RX", value: (point) => point.networkRxAverage, summaryAggregation: "average", showInTooltip: true },
            { label: "TX", value: (point) => point.networkTxAverage, summaryAggregation: "average", showInTooltip: true, color: COLORS[1] },
          ]} />
        </div>
      </section>

      <section className="observability-section" aria-labelledby="service-history-heading">
        <div className="section-heading">
          <div>
            <p className="eyebrow">SERVICES</p>
            <h2 id="service-history-heading">Platform availability</h2>
          </div>
        </div>
        <div className="service-history-grid">
          {services.map((service) => <ServiceHistory key={service.id} service={service} />)}
          {services.length === 0 ? <div className="chart-empty">No service samples in this range</div> : null}
        </div>
      </section>

      <section className="observability-section event-history" aria-labelledby="event-history-heading">
        <div className="section-heading">
          <div>
            <p className="eyebrow">EVENTS</p>
            <h2 id="event-history-heading">Infrastructure timeline</h2>
          </div>
          <span className="history-window">Events retained long-term</span>
        </div>
        <div className="observability-events">
          {events.map((event) => (
            <article className="observability-event" key={event.id}>
              <time dateTime={event.occurredAt}>{eventTime(event.occurredAt)}</time>
              <div>
                <span>{eventLabel(event.type)}</span>
                <strong>{event.resourceName || event.resourceId || "Infrastructure"}</strong>
                <p>{event.summary}</p>
              </div>
            </article>
          ))}
          {events.length === 0 ? <div className="chart-empty">No infrastructure events in this range</div> : null}
        </div>
      </section>
    </>
  )
}
