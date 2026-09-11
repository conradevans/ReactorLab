import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import DeploymentsPage from "./DeploymentsPage"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("DeploymentsPage", () => {
  test("renders the updated administrator response shape", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        deployments: [
          {
            app: "hello-minideploy",
            strategy: "node-express",
            status: "healthy",
            containers: [
              {
                service: "app",
                container: "hello-minideploy",
                cpuPercent: 0.3,
                memoryUsedBytes: 1048576,
                networkRxBytes: 512,
                networkTxBytes: 256,
                restartCount: 0,
              },
            ],
            database: { state: "detached" },
          },
        ],
        collectedAt: "2026-09-11T04:00:00Z",
      }),
    }))

    render(<DeploymentsPage navigate={vi.fn()} />)

    expect(await screen.findByText("hello-minideploy")).toBeTruthy()
    expect(screen.getByText("node-express")).toBeTruthy()
    expect(screen.getByText("Live · refreshes every 1s")).toBeTruthy()
    expect(screen.queryByText("No deployments")).toBeNull()
  })

  test("shows collection progress instead of a blank list", () => {
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})))
    render(<DeploymentsPage navigate={vi.fn()} />)
    expect(screen.getByText("Loading deployments…")).toBeTruthy()
  })
})
