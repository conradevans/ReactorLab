import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import ObservabilityPage from "./ObservabilityPage"

const timestamp = "2026-09-17T12:00:00Z"

function envelope(data) {
  return {
    range: "1h",
    from: "2026-09-17T11:00:00Z",
    to: timestamp,
    data,
  }
}

function response(body, ok = true) {
  return Promise.resolve({
    ok,
    status: ok ? 200 : 503,
    json: vi.fn().mockResolvedValue(body),
  })
}

function stubObservability({ empty = false, fail = false } = {}) {
  const hostPoint = {
    timestamp,
    sampleCount: 1,
    cpuAverage: 30,
    cpuMaximum: 70,
    memoryUsedAverage: 50,
    memoryUsedMaximum: 60,
    memoryTotal: 100,
    load1Average: 1,
    load5Average: 0.8,
    load15Average: 0.5,
    diskUsedAverage: 40,
    diskTotal: 100,
    diskReadAverage: 1024,
    diskWriteAverage: 2048,
    networkRxAverage: 4096,
    networkTxAverage: 8192,
  }
  const appPoint = {
    timestamp,
    sampleCount: 1,
    cpuAverage: 12,
    cpuMaximum: 22,
    memoryUsedAverage: 1024,
    memoryUsedMaximum: 2048,
    memoryLimitAverage: 4096,
    networkRxAverage: 512,
    networkTxAverage: 256,
    status: "healthy",
    restartCount: 2,
  }
  const fetch = vi.fn().mockImplementation((path) => {
    if (fail) return response({}, false)
    if (path.includes("/host?")) return response(envelope({ points: empty ? [] : [hostPoint] }))
    if (path.includes("/temperature?")) {
      return response(envelope({ points: empty ? [] : [{
        bucketStart: timestamp,
        bucketEnd: timestamp,
        sampleCount: 5,
        minCelsius: 58,
        avgCelsius: 72.2,
        maxCelsius: 91,
        peakAt: timestamp,
      }] }))
    }
    if (path.match(/\/apps\?range=/)) {
      return response(envelope({ applications: empty ? [] : [
        { id: "alpha", name: "Alpha", latestStatus: "healthy", restartCount: 2, lastObservedAt: timestamp },
        { id: "beta", name: "Beta", latestStatus: "stopped", restartCount: 0, lastObservedAt: timestamp },
      ] }))
    }
    if (path.includes("/apps/")) return response(envelope({ points: empty ? [] : [appPoint] }))
    if (path.includes("/services?")) {
      return response(envelope({ services: empty ? [] : [{
        id: "minibase",
        name: "MiniBase",
        points: [{ timestamp, sampleCount: 1, available: true, status: "healthy" }],
      }] }))
    }
    if (path.includes("/events?")) {
      return response(envelope({ events: empty ? [] : [{
        id: "event-1",
        type: "database_backup",
        resourceName: "Primary",
        occurredAt: timestamp,
        summary: "Backup created",
      }] }))
    }
    return Promise.reject(new Error(`Unexpected request: ${path}`))
  })
  vi.stubGlobal("fetch", fetch)
  return fetch
}

function inspectChart(title) {
  const heading = screen.getByRole("heading", { name: title, level: 3 })
  const card = heading.closest("article")
  const target = within(card).getByTestId("chart-inspection-target")
  fireEvent.pointerMove(target, { clientX: 340, pointerType: "mouse" })
  return {
    card,
    tooltip: within(card).getByTestId("chart-inspection-tooltip"),
  }
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("ObservabilityPage", () => {
  test("renders bounded controls, host charts, temperature peak, apps, services, and events", async () => {
    const fetch = stubObservability()
    render(<ObservabilityPage />)

    expect(await screen.findByText("Dell history")).toBeTruthy()
    for (const label of ["15M", "1H", "6H", "24H", "7D"]) {
      expect(screen.getByRole("button", { name: label })).toBeTruthy()
    }
    for (const chart of [
      "CPU",
      "Memory",
      "Temperature",
      "Load",
      "Disk capacity",
      "Disk I/O",
      "Network",
      "Application CPU",
      "Application memory",
      "Application network",
    ]) {
      expect(screen.getByText(chart)).toBeTruthy()
    }
    expect(screen.getByText("Peak 91.0°C")).toBeTruthy()
    expect(await screen.findByRole("option", { name: "Alpha" })).toBeTruthy()
    expect(await screen.findByText("Status history")).toBeTruthy()
    expect(screen.getByText("Restarts 2 → 2")).toBeTruthy()
    expect(screen.getByText("MiniBase")).toBeTruthy()
    expect(screen.getByText("Backup created")).toBeTruthy()
    expect(fetch.mock.calls.some(([path]) => path.includes("/apps/alpha?range=1h"))).toBe(true)
  })

  test("uses primary point-in-time values and retains genuinely distinct tooltip metrics", async () => {
    stubObservability()
    render(<ObservabilityPage />)
    await screen.findByText("Restarts 2 → 2")

    const primaryCharts = [
      ["CPU", "CPU", "Maximum"],
      ["Memory", "Memory", "Maximum"],
      ["Temperature", "Temperature", "Peak"],
      ["Application CPU", "CPU", "Maximum"],
    ]
    for (const [title, primaryLabel, excludedLabel] of primaryCharts) {
      const { card, tooltip } = inspectChart(title)
      expect(within(tooltip).getByText(primaryLabel)).toBeTruthy()
      expect(within(tooltip).queryByText(excludedLabel)).toBeNull()
      expect(card.querySelectorAll("path.chart-line")).toHaveLength(2)
      expect(tooltip.querySelectorAll("dl > div")).toHaveLength(1)
    }

    const distinctCharts = [
      ["Load", ["1m", "5m", "15m"]],
      ["Disk capacity", ["Used"]],
      ["Disk I/O", ["Read", "Write"]],
      ["Network", ["RX", "TX"]],
      ["Application network", ["RX", "TX"]],
    ]
    for (const [title, labels] of distinctCharts) {
      const { tooltip } = inspectChart(title)
      for (const label of labels) {
        expect(within(tooltip).getByText(label)).toBeTruthy()
      }
      expect(tooltip.querySelectorAll("dl > div")).toHaveLength(labels.length)
    }

    const { tooltip: memoryTooltip } = inspectChart("Application memory")
    for (const label of ["Used", "Limit"]) {
      expect(within(memoryTooltip).getByText(label)).toBeTruthy()
    }
    expect(within(memoryTooltip).queryByText("Maximum")).toBeNull()
    expect(memoryTooltip.querySelectorAll("dl > div")).toHaveLength(2)
  })

  test("range and application changes issue only bounded preset requests", async () => {
    const fetch = stubObservability()
    render(<ObservabilityPage />)
    await screen.findByRole("option", { name: "Alpha" })

    fireEvent.click(screen.getByRole("button", { name: "7D" }))
    expect(await screen.findByText("7D ending now")).toBeTruthy()
    expect(fetch.mock.calls.some(([path]) => path === "/api/v1/observability/host?range=7d")).toBe(true)

    fireEvent.change(screen.getByLabelText("Application"), { target: { value: "beta" } })
    await waitFor(() => {
      expect(fetch.mock.calls.some(([path]) => path.includes("/apps/beta?range=7d"))).toBe(true)
    })
  })

  test("renders explicit empty and unavailable states", async () => {
    stubObservability({ empty: true })
    render(<ObservabilityPage />)
    expect(await screen.findByText("No applications")).toBeTruthy()
    expect(screen.getAllByText("No samples in this range").length).toBeGreaterThan(0)
    expect(screen.getByText("No service samples in this range")).toBeTruthy()
    expect(screen.getByText("No infrastructure events in this range")).toBeTruthy()

    cleanup()
    stubObservability({ fail: true })
    render(<ObservabilityPage />)
    expect(await screen.findByText("Historical observability is unavailable.")).toBeTruthy()
  })
})
