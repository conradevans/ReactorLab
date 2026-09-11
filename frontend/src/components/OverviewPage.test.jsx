import { cleanup, fireEvent, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import OverviewPage from "./OverviewPage"

const systemFixture = {
  cpu: {
    usagePercent: 41,
    logicalCores: 16,
    load1: 14,
    load5: 12,
    load15: 8,
  },
  memory: { usagePercent: 74 },
  disk: { usagePercent: 90 },
  temperature: { celsius: 80, source: "coretemp" },
  uptimeSeconds: 3600,
  battery: {
    available: true,
    percent: 100,
    acAvailable: true,
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
}

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("OverviewPage", () => {
  test("renders five neutral metric cards with current dots and accessible help", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue(systemFixture),
    }))

    const view = render(<OverviewPage />)

    expect(await screen.findByText("100%")).toBeTruthy()
    expect(screen.getByText("Full · AC power")).toBeTruthy()
    expect(
      view.container.querySelectorAll(".overview-stat-card"),
    ).toHaveLength(5)
    expect(screen.getByRole("button", { name: "About CPU" })).toBeTruthy()
    expect(screen.getByRole("button", { name: "About Memory" })).toBeTruthy()
    expect(screen.getByRole("button", { name: "About Disk" })).toBeTruthy()
    expect(
      screen.getByRole("button", { name: "About Temperature" }),
    ).toBeTruthy()
    expect(screen.getByRole("button", { name: "About Battery" })).toBeTruthy()
    expect(screen.getAllByLabelText("Normal status")).toHaveLength(2)
    expect(screen.getAllByLabelText("Warning status")).toHaveLength(2)
    expect(screen.getAllByLabelText("Critical status")).toHaveLength(1)

    const cpuInfo = screen.getByRole("button", { name: "About CPU" })
    fireEvent.click(cpuInfo)
    expect(screen.getByText(/1- and 5-minute load averages/)).toBeTruthy()
  })
})
