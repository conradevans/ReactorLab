import { describe, expect, it } from "vitest"

import {
  activityKindLabel,
  activitySeverityClass,
  activitySourceLabel,
  formatActivityTime,
} from "./activityMetrics"

describe("activity formatting", () => {
  it("formats kinds and sources", () => {
    expect(activityKindLabel("warning")).toBe("Warning")
    expect(activityKindLabel("recovery")).toBe("Recovered")
    expect(activitySourceLabel("minideploy")).toBe("MiniDeploy")
    expect(activitySourceLabel("minibase")).toBe("MiniBase")
    expect(activitySourceLabel("system")).toBe("Dell")
  })

  it("maps severities to safe visual classes", () => {
    expect(activitySeverityClass("warning")).toBe("activity-severity-warning")
    expect(activitySeverityClass("error")).toBe("activity-severity-error")
    expect(activitySeverityClass("info")).toBe("activity-severity-info")
    expect(activitySeverityClass("other")).toBe("activity-severity-neutral")
  })

  it("handles missing and invalid timestamps", () => {
    expect(formatActivityTime()).toBe("—")
    expect(formatActivityTime("not-a-date")).toBe("—")
  })
})
