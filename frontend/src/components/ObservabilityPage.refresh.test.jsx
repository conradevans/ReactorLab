import { act, cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import {
  getApplicationObservability,
  getObservabilityOverview,
} from "../observabilityApi"
import ObservabilityPage from "./ObservabilityPage"

vi.mock("../observabilityApi", () => ({
  OBSERVABILITY_RANGES: ["15m", "1h", "6h", "24h", "7d"],
  getObservabilityOverview: vi.fn(),
  getApplicationObservability: vi.fn(),
}))

const timestamp = "2026-09-18T12:00:00Z"

function deferred() {
  let resolve
  let reject
  const promise = new Promise((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

function overview(applications = [
  {
    id: "alpha",
    name: "Alpha",
    latestStatus: "healthy",
    restartCount: 2,
    lastObservedAt: timestamp,
  },
]) {
  return {
    host: [],
    temperature: [],
    applications,
    services: [],
    events: [],
    from: "2026-09-18T11:00:00Z",
    to: timestamp,
  }
}

function appHistory(restartCount) {
  return [{
    timestamp,
    sampleCount: 1,
    cpuAverage: 1,
    cpuMaximum: 2,
    memoryUsedAverage: 3,
    memoryUsedMaximum: 4,
    memoryLimitAverage: 5,
    networkRxAverage: 6,
    networkTxAverage: 7,
    status: "healthy",
    restartCount,
  }]
}

async function flushRequests() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
  })
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  vi.useRealTimers()
})

describe("ObservabilityPage serialized refresh", () => {
  test("does not overlap a pending cycle and refreshes the selected app once per completed cycle", async () => {
    vi.useFakeTimers()
    const firstOverview = deferred()
    getObservabilityOverview
      .mockReturnValueOnce(firstOverview.promise)
      .mockResolvedValue(overview())
    getApplicationObservability.mockResolvedValue(appHistory(2))

    const { unmount } = render(<ObservabilityPage />)
    expect(getObservabilityOverview).toHaveBeenCalledTimes(1)

    await act(async () => {
      vi.advanceTimersByTime(60_000)
    })
    expect(getObservabilityOverview).toHaveBeenCalledTimes(1)
    expect(getApplicationObservability).not.toHaveBeenCalled()

    await act(async () => {
      firstOverview.resolve(overview())
      await firstOverview.promise
    })
    await flushRequests()
    expect(getApplicationObservability).toHaveBeenCalledTimes(1)
    expect(vi.getTimerCount()).toBe(1)

    await act(async () => {
      vi.advanceTimersByTime(29_999)
    })
    expect(getObservabilityOverview).toHaveBeenCalledTimes(1)

    await act(async () => {
      vi.advanceTimersByTime(1)
    })
    await flushRequests()
    expect(getObservabilityOverview).toHaveBeenCalledTimes(2)
    expect(getApplicationObservability).toHaveBeenCalledTimes(2)
    expect(getApplicationObservability.mock.calls.map(([appID]) => appID)).toEqual([
      "alpha",
      "alpha",
    ])
    expect(vi.getTimerCount()).toBe(1)

    const latestSignal = getObservabilityOverview.mock.calls.at(-1)[1].signal
    unmount()
    expect(latestSignal.aborted).toBe(true)
    expect(vi.getTimerCount()).toBe(0)
  })

  test("range changes abort the old cycle and stale overview responses cannot overwrite the new range", async () => {
    vi.useFakeTimers()
    const oldOverview = deferred()
    getObservabilityOverview
      .mockReturnValueOnce(oldOverview.promise)
      .mockResolvedValueOnce(overview([{
        id: "beta",
        name: "Beta",
        latestStatus: "healthy",
        restartCount: 7,
        lastObservedAt: timestamp,
      }]))
    getApplicationObservability.mockResolvedValue(appHistory(7))

    const { unmount } = render(<ObservabilityPage />)
    const oldSignal = getObservabilityOverview.mock.calls[0][1].signal

    fireEvent.click(screen.getByRole("button", { name: "7D" }))
    await flushRequests()

    expect(oldSignal.aborted).toBe(true)
    expect(getObservabilityOverview.mock.calls[1][0]).toBe("7d")
    expect(screen.getByRole("option", { name: "Beta" })).toBeTruthy()
    expect(screen.getByText("Restarts 7 → 7")).toBeTruthy()

    await act(async () => {
      oldOverview.resolve(overview([{
        id: "stale",
        name: "Stale",
        latestStatus: "unavailable",
        restartCount: 99,
        lastObservedAt: timestamp,
      }]))
      await oldOverview.promise
    })
    expect(screen.queryByRole("option", { name: "Stale" })).toBeNull()
    expect(screen.getByRole("option", { name: "Beta" })).toBeTruthy()

    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  test("app changes abort old detail requests and stale detail cannot replace the newer selection", async () => {
    vi.useFakeTimers()
    const alphaHistory = deferred()
    getObservabilityOverview.mockResolvedValue(overview([
      {
        id: "alpha",
        name: "Alpha",
        latestStatus: "healthy",
        restartCount: 2,
        lastObservedAt: timestamp,
      },
      {
        id: "beta",
        name: "Beta",
        latestStatus: "healthy",
        restartCount: 9,
        lastObservedAt: timestamp,
      },
    ]))
    getApplicationObservability.mockImplementation((appID) => (
      appID === "alpha" ? alphaHistory.promise : Promise.resolve(appHistory(9))
    ))

    const { unmount } = render(<ObservabilityPage />)
    await flushRequests()
    expect(getApplicationObservability.mock.calls[0][0]).toBe("alpha")
    const alphaSignal = getApplicationObservability.mock.calls[0][2].signal

    fireEvent.change(screen.getByLabelText("Application"), {
      target: { value: "beta" },
    })
    await flushRequests()

    expect(alphaSignal.aborted).toBe(true)
    expect(getApplicationObservability.mock.calls.filter(([appID]) => appID === "beta")).toHaveLength(1)
    expect(screen.getByText("Restarts 9 → 9")).toBeTruthy()

    await act(async () => {
      alphaHistory.resolve(appHistory(2))
      await alphaHistory.promise
    })
    expect(screen.getByText("Restarts 9 → 9")).toBeTruthy()
    expect(screen.queryByText("Restarts 2 → 2")).toBeNull()

    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  test("a failed periodic app refresh clears stale app history and keeps a single timer", async () => {
    vi.useFakeTimers()
    getObservabilityOverview.mockResolvedValue(overview())
    getApplicationObservability
      .mockResolvedValueOnce(appHistory(2))
      .mockRejectedValueOnce(new Error("unavailable"))

    const { unmount } = render(<ObservabilityPage />)
    await flushRequests()
    expect(screen.getByText("Restarts 2 → 2")).toBeTruthy()

    await act(async () => {
      vi.advanceTimersByTime(30_000)
    })
    await flushRequests()

    expect(getApplicationObservability).toHaveBeenCalledTimes(2)
    expect(screen.getByText("Application history is unavailable.")).toBeTruthy()
    expect(screen.queryByText("Restarts 2 → 2")).toBeNull()
    expect(vi.getTimerCount()).toBe(1)

    unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  test("unmount aborts a pending first request and leaves no timer", async () => {
    vi.useFakeTimers()
    const pending = deferred()
    getObservabilityOverview.mockReturnValue(pending.promise)

    const { unmount } = render(<ObservabilityPage />)
    const signal = getObservabilityOverview.mock.calls[0][1].signal
    unmount()

    expect(signal.aborted).toBe(true)
    expect(vi.getTimerCount()).toBe(0)

    await act(async () => {
      pending.resolve(overview())
      await pending.promise
    })
    expect(getApplicationObservability).not.toHaveBeenCalled()
  })
})
