import { describe, expect, test } from "vitest"

import {
  batterySeverity,
  cpuLoadSeverity,
  cpuUsageSeverity,
  diskSeverity,
  memorySeverity,
  serviceSeverity,
  temperatureSeverity,
} from "./systemStatus"

describe("system metric severity thresholds", () => {
  test.each([
    [79.9, "healthy"],
    [80, "warning"],
    [94.9, "warning"],
    [95, "critical"],
  ])("classifies temperature %s", (value, expected) => {
    expect(temperatureSeverity(value)).toBe(expected)
  })

  test.each([
    [74.9, "healthy"],
    [75, "warning"],
    [89.9, "warning"],
    [90, "critical"],
  ])("classifies memory %s", (value, expected) => {
    expect(memorySeverity(value)).toBe(expected)
  })

  test.each([
    [79.9, "healthy"],
    [80, "warning"],
    [89.9, "warning"],
    [90, "critical"],
  ])("classifies disk %s", (value, expected) => {
    expect(diskSeverity(value)).toBe(expected)
  })

  test("does not infer CPU utilization severity", () => {
    expect(cpuUsageSeverity(100)).toBeNull()
  })

  test("classifies normalized 1- and 5-minute CPU load conservatively", () => {
    expect(cpuLoadSeverity({
      logicalCores: 16,
      load1: 13.59,
      load5: 10,
    })).toBe("healthy")
    expect(cpuLoadSeverity({
      logicalCores: 16,
      load1: 13.6,
      load5: 10,
    })).toBe("warning")
    expect(cpuLoadSeverity({ logicalCores: 16, load1: 20, load5: 8 })).toBe(
      "critical",
    )
    expect(cpuLoadSeverity({ logicalCores: 0, load1: 20 })).toBeNull()
  })

  test("uses current AC and service state without persistence", () => {
    expect(batterySeverity({ available: true, acConnected: false })).toBe(
      "critical",
    )
    expect(batterySeverity({ available: true, acConnected: true })).toBe(
      "healthy",
    )
    expect(serviceSeverity({ active: false, status: "restarting" })).toBe(
      "warning",
    )
    expect(serviceSeverity({ active: false, status: "failed" })).toBe(
      "critical",
    )
  })
})
