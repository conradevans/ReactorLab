import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, test } from "vitest"

import {
  formatInspectionTimestamp,
  inspectChartPosition,
  interpolateChartValue,
  prepareChartPoints,
} from "../chartInspection"
import { Chart } from "./ObservabilityPage"

const start = Date.parse("2026-09-19T15:37:40Z")
const end = start + 8_000
const points = [
  { timestamp: new Date(start).toISOString(), average: 0, peak: 5 },
  { timestamp: new Date(end).toISOString(), average: 80, peak: 45 },
]
const series = [
  { label: "Average", value: (point) => point.average },
  { label: "Peak", value: (point) => point.peak, color: "#fb7185" },
]
const formatValue = (value) => `${value.toFixed(1)} widgets`

function renderChart(chartPoints = points) {
  const view = render(
    <Chart
      description="Inspectable values"
      formatValue={formatValue}
      points={chartPoints}
      series={series}
      title="Inspection test"
    />,
  )
  const svg = screen.getByRole("img", { name: "Inspection test historical chart" })
  svg.getBoundingClientRect = () => ({
    bottom: 190,
    height: 190,
    left: 0,
    right: 680,
    top: 0,
    width: 680,
    x: 0,
    y: 0,
    toJSON: () => ({}),
  })
  return {
    ...view,
    svg,
    target: screen.getByTestId("chart-inspection-target"),
  }
}

afterEach(() => {
  cleanup()
})

describe("chart inspection calculations", () => {
  test("maps midpoint and quarter positions to continuous cursor timestamps without snapping", () => {
    const rect = { left: 100, width: 340 }
    const midpoint = inspectChartPosition(270, rect, start, end)
    const quarter = inspectChartPosition(188, rect, start, end)

    expect(midpoint.x).toBe(340)
    expect(midpoint.normalized).toBe(0.5)
    expect(midpoint.timestamp).toBe(start + 4_000)

    expect(quarter.x).toBe(176)
    expect(quarter.normalized).toBe(0.25)
    expect(quarter.timestamp).toBe(start + 2_000)
    expect(quarter.x).not.toBe(12)
    expect(quarter.x).not.toBe(668)
  })

  test("clamps cursor mapping at both plot edges", () => {
    const rect = { left: 0, width: 680 }
    const left = inspectChartPosition(-100, rect, start, end)
    const right = inspectChartPosition(1_000, rect, start, end)

    expect(left).toEqual({ normalized: 0, timestamp: start, x: 12 })
    expect(right).toEqual({ normalized: 1, timestamp: end, x: 668 })
  })

  test("sorts a defensive copy and interpolates exact, midpoint, and independent series values", () => {
    const unsorted = [points[1], points[0]]
    const entries = prepareChartPoints(unsorted)

    expect(unsorted[0]).toBe(points[1])
    expect(entries.map((entry) => entry.timestamp)).toEqual([start, end])
    expect(interpolateChartValue(entries, series[0].value, start)).toBe(0)
    expect(interpolateChartValue(entries, series[0].value, start + 4_000)).toBe(40)
    expect(interpolateChartValue(entries, series[1].value, start + 4_000)).toBe(25)
  })

  test("does not interpolate across missing values or disconnected gaps", () => {
    const entries = prepareChartPoints([
      { timestamp: new Date(start).toISOString(), value: 10 },
      { timestamp: new Date(start + 4_000).toISOString(), value: null },
      { timestamp: new Date(end).toISOString(), value: 30 },
    ])
    const value = (point) => point.value

    expect(interpolateChartValue(entries, value, start + 2_000)).toBeNull()
    expect(interpolateChartValue(entries, value, start + 6_000)).toBeNull()
  })

  test("handles duplicate timestamps without NaN or Infinity", () => {
    const entries = prepareChartPoints([
      { timestamp: new Date(start).toISOString(), value: null },
      { timestamp: new Date(start).toISOString(), value: 20 },
      { timestamp: new Date(end).toISOString(), value: 40 },
    ])
    const value = (point) => point.value
    const exact = interpolateChartValue(entries, value, start)
    const midpoint = interpolateChartValue(entries, value, start + 4_000)

    expect(exact).toBe(20)
    expect(midpoint).toBe(30)
    expect(Number.isFinite(exact)).toBe(true)
    expect(Number.isFinite(midpoint)).toBe(true)
  })

  test("formats inspection timestamps with date and seconds", () => {
    const formatted = formatInspectionTimestamp(start + 2_000, "en-US")

    expect(formatted).toContain("Sep 19, 2026")
    expect(formatted).toMatch(/:37:42/)
  })
})

describe("Chart pointer interaction", () => {
  test("mouse movement shows a continuously positioned crosshair and formatted interpolation", () => {
    const { target } = renderChart()

    fireEvent.pointerEnter(target, { clientX: 176, pointerType: "mouse" })

    expect(screen.getByTestId("chart-crosshair").getAttribute("x1")).toBe("176")
    expect(screen.getByTestId("chart-inspection-tooltip").className).toContain("align-right")
    expect(screen.getByTestId("chart-inspection-time").textContent).toMatch(/:37:42/)
    expect(screen.getByText("15.0 widgets")).toBeTruthy()
    expect(screen.getByText("20.0 widgets")).toBeTruthy()
    expect(screen.getByText("Average 80.0 widgets")).toBeTruthy()

    fireEvent.pointerMove(target, { clientX: 504, pointerType: "mouse" })

    expect(screen.getByTestId("chart-crosshair").getAttribute("x1")).toBe("504")
    expect(screen.getByTestId("chart-inspection-tooltip").className).toContain("align-left")
    expect(screen.getByText("60.0 widgets")).toBeTruthy()
    expect(screen.getByText("35.0 widgets")).toBeTruthy()
  })

  test("renders an unavailable value instead of bridging a series gap", () => {
    const gapPoints = [
      points[0],
      {
        timestamp: new Date(start + 4_000).toISOString(),
        average: null,
        peak: 25,
      },
      points[1],
    ]
    const { target } = renderChart(gapPoints)

    fireEvent.pointerMove(target, { clientX: 176, pointerType: "mouse" })

    expect(screen.getByText("—")).toBeTruthy()
    expect(screen.getByText("15.0 widgets")).toBeTruthy()
  })

  test("mouse leave hides hover inspection", () => {
    const { target } = renderChart()

    fireEvent.pointerMove(target, { clientX: 340, pointerType: "mouse" })
    expect(screen.getByTestId("chart-inspection-tooltip")).toBeTruthy()

    fireEvent.pointerLeave(target, { pointerType: "mouse" })
    expect(screen.queryByTestId("chart-inspection-tooltip")).toBeNull()
    expect(screen.queryByTestId("chart-crosshair")).toBeNull()
  })

  test("touch taps pin inspection and a second tap moves it", () => {
    const { target } = renderChart()

    fireEvent.pointerDown(target, { clientX: 176, pointerType: "touch" })
    expect(screen.getByTestId("chart-crosshair").getAttribute("x1")).toBe("176")

    fireEvent.pointerLeave(target, { pointerType: "touch" })
    expect(screen.getByTestId("chart-inspection-tooltip")).toBeTruthy()

    fireEvent.pointerDown(target, { clientX: 504, pointerType: "touch" })
    expect(screen.getByTestId("chart-crosshair").getAttribute("x1")).toBe("504")
    expect(screen.getByText("60.0 widgets")).toBeTruthy()
  })

  test("new chart data clears a touch-pinned inspection while preserving latest values", async () => {
    const { rerender, target } = renderChart()

    fireEvent.pointerDown(target, { clientX: 340, pointerType: "pen" })
    expect(screen.getByTestId("chart-inspection-tooltip")).toBeTruthy()
    expect(screen.getByText("Average 80.0 widgets")).toBeTruthy()

    const nextPoints = points.map((point) => ({
      ...point,
      timestamp: new Date(Date.parse(point.timestamp) + 60_000).toISOString(),
    }))
    rerender(
      <Chart
        description="Inspectable values"
        formatValue={formatValue}
        points={nextPoints}
        series={series}
        title="Inspection test"
      />,
    )

    await waitFor(() => {
      expect(screen.queryByTestId("chart-inspection-tooltip")).toBeNull()
    })
    expect(screen.getByText("Average 80.0 widgets")).toBeTruthy()
  })
})
