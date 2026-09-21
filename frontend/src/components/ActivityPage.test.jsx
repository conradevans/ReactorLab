import { cleanup, render, screen, waitFor } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest"

import ActivityPage from "./ActivityPage"

const eventID = "b".repeat(64)
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

function response(events) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue({ events }),
  }
}

beforeEach(() => {
  Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
    configurable: true,
    value: vi.fn(),
  })
})

afterEach(() => {
  cleanup()
  delete HTMLElement.prototype.scrollIntoView
  vi.unstubAllGlobals()
})

describe("Activity recovery incidents", () => {
  test("renders, focuses, and highlights a deep-linked recovery incident", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response([recoveryEvent]))
    vi.stubGlobal("fetch", fetchMock)

    render(<ActivityPage eventID={eventID} />)

    expect(await screen.findByText("Unexpected shutdown detected")).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/activity?event=${eventID}`,
      expect.objectContaining({ cache: "no-store" }),
    )
    expect(screen.getByText("Last known alive")).toBeTruthy()
    expect(screen.getByText("Downtime")).toBeTruthy()
    expect(screen.getByText("4m 24s")).toBeTruthy()
    expect(screen.getByText("Recovery incident")).toBeTruthy()
    expect(
      screen.getByText(
        "7 day operational activity · recovery incidents retained",
      ),
    ).toBeTruthy()

    const row = document.getElementById(`activity-event-${eventID}`)
    expect(row).toBeTruthy()
    expect(document.activeElement).toBe(row)
    await waitFor(() => {
      expect(row.classList.contains("activity-row-selected")).toBe(true)
    })
    expect(row.scrollIntoView).toHaveBeenCalledWith({
      behavior: "smooth",
      block: "center",
    })
  })

  test("ignores a malformed event ID and loads normal Activity", async () => {
    const fetchMock = vi.fn().mockResolvedValue(response([{
      id: 7,
      occurredAt: "2026-09-21T10:00:00Z",
      source: "system",
      kind: "warning",
      severity: "warning",
      subject: "Legacy activity",
      message: "Still available.",
    }]))
    vi.stubGlobal("fetch", fetchMock)

    render(<ActivityPage eventID="../../private" />)

    expect(await screen.findByText("Legacy activity")).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/v1/activity",
      expect.objectContaining({ cache: "no-store" }),
    )
    expect(document.querySelector(".activity-row-selected")).toBeNull()
  })

  test("handles a valid but nonexistent event ID normally", async () => {
    const missingID = "c".repeat(64)
    const fetchMock = vi.fn().mockResolvedValue(response([]))
    vi.stubGlobal("fetch", fetchMock)

    render(<ActivityPage eventID={missingID} />)

    expect(await screen.findByText("No monitoring events yet")).toBeTruthy()
    expect(fetchMock).toHaveBeenCalledWith(
      `/api/v1/activity?event=${missingID}`,
      expect.objectContaining({ cache: "no-store" }),
    )
    expect(document.getElementById(`activity-event-${missingID}`)).toBeNull()
  })
})
