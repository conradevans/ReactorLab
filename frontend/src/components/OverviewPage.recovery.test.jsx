import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import OverviewPage from "./OverviewPage"

const eventID = "a".repeat(64)

function systemFixture(recovery) {
  return {
    cpu: {
      usagePercent: 10,
      logicalCores: 8,
      load1: 0.01,
      load5: 0.05,
      load15: 0.07,
    },
    memory: { usagePercent: 20 },
    disk: { usagePercent: 30 },
    temperature: { celsius: 55, source: "coretemp" },
    uptimeSeconds: 10380,
    battery: {
      available: true,
      percent: 100,
      acConnected: true,
      status: "Full",
    },
    services: [
      {
        name: "ReactorLab",
        unit: "reactorlab.service",
        status: "active",
        active: true,
      },
    ],
    ...(recovery ? { recovery } : {}),
  }
}

function response(body) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(body),
  }
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("Overview recovery status", () => {
  test.each([
    ["armed", "Armed", "service-up"],
    ["not_armed", "Not armed", "recovery-warning"],
    ["unavailable", "Unavailable", "recovery-unavailable"],
  ])("renders %s protection state", async (state, label, className) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(systemFixture({
      protection: {
        state,
        hardwareWatchdog: { state: "unavailable" },
        rtc: { state: "unavailable" },
      },
      historyAvailable: true,
      lastIncident: null,
    }))))

    render(<OverviewPage />)

    const value = await screen.findByText(label)
    expect(value.classList.contains(className)).toBe(true)
    expect(screen.getByText("Automatic recovery")).toBeTruthy()
  })
  test.each([
    [
      "older Phase 1",
      { state: "armed", wakeAt: "2026-09-21T12:05:00Z" },
      "Armed",
    ],
    [
      "hardware only",
      { hardwareWatchdog: { state: "armed" } },
      "Armed",
    ],
    [
      "RTC only",
      { rtc: { state: "not_armed" } },
      "Not armed",
    ],
    [
      "missing nested states",
      { hardwareWatchdog: {}, rtc: {} },
      "Unavailable",
    ],
  ])("handles %s protection response", async (_name, protection, label) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(systemFixture({
      protection,
      historyAvailable: true,
      lastIncident: null,
    }))))

    render(<OverviewPage />)

    expect(await screen.findByText(label)).toBeTruthy()
  })

  test("handles absent recovery data and no last incident", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(systemFixture())))

    render(<OverviewPage />)

    expect(await screen.findByText("None recorded")).toBeTruthy()
    expect(screen.getByText("Unavailable")).toBeTruthy()
    expect(
      screen.queryByRole("link", { name: /Last recovery/i }),
    ).toBeNull()
  })

  test("formats and navigates to the latest recovery incident", async () => {
    const navigate = vi.fn()
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(systemFixture({
      protection: {
        state: "armed",
        wakeAt: "2026-09-21T12:05:00Z",
      },
      historyAvailable: true,
      lastIncident: {
        eventId: eventID,
        lastKnownAliveAt: "2026-09-21T10:55:36Z",
        recoveredAt: "2026-09-21T11:00:00Z",
        downtimeSeconds: 264,
        status: "recovered",
      },
    }))))

    render(<OverviewPage navigate={navigate} />)

    const link = await screen.findByRole("link")
    expect(link.textContent).toContain("Sep 21 · 4m 24s")
    expect(link.getAttribute("href")).toBe(
      `/admin/activity?event=${eventID}`,
    )
    fireEvent.click(link)
    expect(navigate).toHaveBeenCalledWith(
      `/admin/activity?event=${eventID}`,
    )
  })

  test("leaves the Control plane card structure unchanged", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response(systemFixture({
      protection: { state: "armed" },
      historyAvailable: true,
      lastIncident: null,
    }))))

    render(<OverviewPage />)

    await screen.findByText("Automatic recovery")
    const heading = screen.getByRole("heading", { name: "Control plane" })
    const card = heading.closest("article")
    expect(within(card).getByText("ReactorLab")).toBeTruthy()
    expect(within(card).getByText("Active")).toBeTruthy()
    expect(within(card).queryByText("Automatic recovery")).toBeNull()
    expect(within(card).queryByText("Last recovery")).toBeNull()
  })
})
