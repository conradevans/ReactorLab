import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import DatabasesPage from "./DatabasesPage"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("DatabasesPage", () => {
  test("renders the updated administrator response shape", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        databases: [
          {
            id: "database_0123456789abcdef0123456789abcdef",
            displayName: "MyScheduler Production",
            status: "ready",
            sizeBytes: 2097152,
            connections: 3,
            backupCount: 2,
            backupBytes: 4194304,
            backupAgeSeconds: 3600,
            deployment: { state: "linked", app: "myscheduler" },
          },
        ],
        postgres: {
          state: "running",
          cpuPercent: 0.5,
          memoryUsedBytes: 1048576,
          memoryPercent: 0.2,
          networkRxBytes: 1024,
          networkTxBytes: 512,
          blockReadBytes: 256,
          blockWriteBytes: 128,
          pids: 8,
        },
        collectedAt: "2026-09-11T04:00:00Z",
      }),
    }))

    render(<DatabasesPage navigate={vi.fn()} />)

    expect(await screen.findByText("MyScheduler Production")).toBeTruthy()
    expect(
      screen.getByText(/Deployment ·/).textContent,
    ).toContain("myscheduler")
    expect(screen.getByText("Live · refreshes every 1s")).toBeTruthy()
    expect(screen.queryByText("No databases")).toBeNull()
  })

  test("shows collection progress instead of a blank list", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})))
    render(<DatabasesPage navigate={vi.fn()} />)
    expect(screen.getByText("Loading databases…")).toBeTruthy()
  })
})
