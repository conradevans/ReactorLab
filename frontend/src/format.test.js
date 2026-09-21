import { describe, expect, it } from "vitest"

import {
  formatCompactDate,
  formatDuration,
  formatBytes,
  formatPercent,
  formatRate,
  formatTemperature,
  formatUptime,
} from "./format"

describe("metric formatting", () => {
  it("formats percentages and temperature", () => {
    expect(formatPercent(8.216)).toBe("8.2%")
    expect(formatTemperature(66.875)).toBe("66.9°C")
  })

  it("formats bytes and rates", () => {
    expect(formatBytes(1073741824)).toBe("1.0 GiB")
    expect(formatRate(1048576)).toBe("1.0 MiB/s")
  })

  it("formats uptime", () => {
    expect(formatUptime(11960)).toBe("3h 19m")
    expect(formatUptime(90061)).toBe("1d 1h 1m")
  })

  it("formats recovery durations and compact dates", () => {
    expect(formatDuration(42)).toBe("42s")
    expect(formatDuration(264)).toBe("4m 24s")
    expect(formatDuration(4080)).toBe("1h 8m")
    expect(formatDuration(15840)).toBe("4h 24m")
    expect(formatDuration(-1)).toBe("—")
    expect(formatCompactDate("2026-09-21T12:00:00Z")).toBe("Sep 21")
    expect(formatCompactDate("invalid")).toBe("—")
  })
})
