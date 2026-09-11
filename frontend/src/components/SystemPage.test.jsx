import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import SystemPage from "./SystemPage"

const systemFixture = {
  cpu: {
    usagePercent: 100,
    logicalCores: 8,
    load1: 1,
    load5: 0.5,
    load15: 0.25,
  },
  memory: {
    usagePercent: 75,
    usedBytes: 1024,
    availableBytes: 2048,
    totalBytes: 3072,
    swapUsagePercent: 0,
    swapUsedBytes: 0,
    swapTotalBytes: 0,
  },
  disk: {
    usagePercent: 80,
    usedBytes: 1024,
    availableBytes: 2048,
    totalBytes: 3072,
  },
  temperature: { celsius: 80, source: "k10temp" },
  uptimeSeconds: 3600,
  network: {
    interface: "eth0",
    rxBytes: 1000,
    txBytes: 500,
    rxBytesPerSecond: 100,
    txBytesPerSecond: 50,
  },
  battery: {
    available: true,
    percent: 64,
    acConnected: false,
    status: "Discharging",
  },
  services: [
    { name: "ReactorLab", unit: "reactorlab.service", status: "active", active: true },
  ],
}

function response(body = systemFixture) {
  return {
    ok: true,
    json: vi.fn().mockResolvedValue(body),
  }
}

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe("SystemPage", () => {
  test("renders current battery warning, thresholds, and tap-accessible metric help", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(response()))
    render(<SystemPage />)

    expect(await screen.findByText("Running on battery")).toBeTruthy()
    expect(screen.getByText("64%")).toBeTruthy()
    expect(screen.getAllByLabelText("Warning status").length).toBeGreaterThan(0)
    expect(screen.getAllByLabelText("Normal status").length).toBeGreaterThan(0)
    expect(screen.queryByLabelText("Critical status")).toBeTruthy()

    const infoButton = screen.getByRole("button", { name: "About Usage" })
    fireEvent.click(infoButton)
    expect(infoButton.getAttribute("aria-expanded")).toBe("true")
    expect(
      screen.getByText(/percentage of CPU time currently being used/),
    ).toBeTruthy()
    fireEvent.pointerDown(document.body)
    expect(infoButton.getAttribute("aria-expanded")).toBe("false")
  })

  test("schedules the one-second poll only after the previous request completes", async () => {
    vi.useFakeTimers()
    let resolveFirst
    const fetchMock = vi.fn()
      .mockImplementationOnce(
        () => new Promise((resolve) => {
          resolveFirst = resolve
        }),
      )
      .mockResolvedValue(response())
    vi.stubGlobal("fetch", fetchMock)

    render(<SystemPage />)
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000)
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await act(async () => {
      resolveFirst(response())
      await Promise.resolve()
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(999)
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
