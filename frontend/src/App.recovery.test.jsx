import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest"

import App from "./App"

const eventID = "d".repeat(64)
const recoveryEvent = {
  eventId: eventID,
  occurredAt: "2026-09-21T11:00:00Z",
  source: "reactorlab",
  kind: "unexpected_shutdown_recovery",
  severity: "warning",
  subject: "Unexpected shutdown detected",
  message: "Dell host recovered after an unexpected shutdown.",
  incident: {
    lastKnownAliveAt: "2026-09-21T10:55:36Z",
    recoveredAt: "2026-09-21T11:00:00Z",
    downtimeSeconds: 264,
    status: "recovered",
  },
}

const systemResponse = {
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
  services: [{
    name: "ReactorLab",
    unit: "reactorlab.service",
    status: "active",
    active: true,
  }],
  recovery: {
    protection: { state: "armed" },
    historyAvailable: true,
    lastIncident: {
      eventId: eventID,
      lastKnownAliveAt: "2026-09-21T10:55:36Z",
      recoveredAt: "2026-09-21T11:00:00Z",
      downtimeSeconds: 264,
      status: "recovered",
    },
  },
}

function jsonResponse(body) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(body),
  }
}

function appFetch(path) {
  if (path === "/api/v1/session") {
    return Promise.resolve(jsonResponse({ mode: "local" }))
  }
  if (path === "/api/v1/system") {
    return Promise.resolve(jsonResponse(systemResponse))
  }
  if (path === `/api/v1/activity?event=${eventID}`) {
    return Promise.resolve(jsonResponse({ events: [recoveryEvent] }))
  }
  if (path === "/api/v1/activity") {
    return Promise.resolve(jsonResponse({ events: [] }))
  }
  return Promise.reject(new Error(`unexpected request: ${path}`))
}

beforeEach(() => {
  window.history.replaceState({}, "", "/")
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: vi.fn(),
  })
})

afterEach(() => {
  cleanup()
  window.history.replaceState({}, "", "/")
  delete HTMLElement.prototype.scrollIntoView
  vi.unstubAllGlobals()
})

describe("recovery deep-link routing", () => {
  test("loads an Activity incident directly with its query string", async () => {
    window.history.replaceState(
      {},
      "",
      `/admin/activity?event=${eventID}`,
    )
    const fetchMock = vi.fn(appFetch)
    vi.stubGlobal("fetch", fetchMock)

    render(<App />)

    expect(await screen.findByText("Unexpected shutdown detected")).toBeTruthy()
    expect(window.location.pathname).toBe("/admin/activity")
    expect(window.location.search).toBe(`?event=${eventID}`)
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/activity?event=${eventID}`,
      expect.objectContaining({ cache: "no-store" }),
    )
    await waitFor(() => {
      expect(document.activeElement?.id).toBe(`activity-event-${eventID}`)
    })
  })

  test("preserves the incident query through Overview navigation and popstate", async () => {
    window.history.replaceState({}, "", "/admin")
    vi.stubGlobal("fetch", vi.fn(appFetch))

    render(<App />)

    const link = await screen.findByRole("link", {
      name: /Last recovery/i,
    })
    fireEvent.click(link)

    expect(await screen.findByText("Unexpected shutdown detected")).toBeTruthy()
    expect(window.location.pathname).toBe("/admin/activity")
    expect(window.location.search).toBe(`?event=${eventID}`)

    act(() => {
      window.history.replaceState({}, "", "/admin")
      window.dispatchEvent(new PopStateEvent("popstate"))
    })
    expect(
      await screen.findByRole("heading", { name: "System health" }),
    ).toBeTruthy()

    act(() => {
      window.history.replaceState(
        {},
        "",
        `/admin/activity?event=${eventID}`,
      )
      window.dispatchEvent(new PopStateEvent("popstate"))
    })
    expect(await screen.findByText("Unexpected shutdown detected")).toBeTruthy()
    expect(window.location.search).toBe(`?event=${eventID}`)
  })
})
