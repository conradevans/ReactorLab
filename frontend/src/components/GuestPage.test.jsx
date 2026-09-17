import { cleanup, render, screen, within } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import GuestPage from "./GuestPage"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function responseWith(overrides = {}) {
  return {
    deployments: {
      available: true,
      summary: { total: 3, shared: 1, hidden: 2 },
      items: [
        {
          app: "portfolio",
          url: "https://portfolio.reactorlab.dev",
          status: "running",
        },
      ],
      ...overrides.deployments,
    },
    databases: {
      available: true,
      summary: { total: 2, shared: 1, hidden: 1 },
      items: [
        {
          id: "database_a",
          displayName: "Shared Database",
          status: "ready",
        },
      ],
      ...overrides.databases,
    },
  }
}

function stubResources(value) {
  const fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: vi.fn().mockResolvedValue(value),
  })
  vi.stubGlobal("fetch", fetch)
  return fetch
}

describe("GuestPage", () => {
  test("renders deployments first, databases second, and exact summaries", async () => {
    const fetch = stubResources(responseWith())
    const { container } = render(<GuestPage navigate={vi.fn()} />)

    expect(await screen.findByText("portfolio")).toBeTruthy()
    expect(screen.getByText("Shared Database")).toBeTruthy()

    const sections = container.querySelectorAll("[data-guest-resource-section]")
    expect(sections).toHaveLength(2)
    expect(sections[0].dataset.guestResourceSection).toBe("deployments")
    expect(sections[1].dataset.guestResourceSection).toBe("databases")

    const deploymentSection = within(sections[0])
    expect(deploymentSection.getByText("Deployments")).toBeTruthy()
    expect(deploymentSection.getByLabelText("Deployments visibility summary").textContent)
      .toContain("TOTAL3SHARED1HIDDEN2")

    const databaseSection = within(sections[1])
    expect(databaseSection.getByText("Databases")).toBeTruthy()
    expect(databaseSection.getByLabelText("Databases visibility summary").textContent)
      .toContain("TOTAL2SHARED1HIDDEN1")

    expect(fetch).toHaveBeenCalledWith("/api/v1/guest/resources", {
      headers: { Accept: "application/json" },
      cache: "no-store",
    })
  })

  test("preserves the approved deployment link and informational database card", async () => {
    stubResources(responseWith())
    render(<GuestPage navigate={vi.fn()} />)

    const appName = await screen.findByText("portfolio")
    const publicURL = screen.getByRole("link", {
      name: "https://portfolio.reactorlab.dev",
    })
    const openLink = screen.getByRole("link", { name: /Open application/ })

    expect(appName).toBeTruthy()
    expect(publicURL.getAttribute("href")).toBe(
      "https://portfolio.reactorlab.dev",
    )
    expect(openLink.getAttribute("href")).toBe(
      "https://portfolio.reactorlab.dev",
    )
    expect(openLink.getAttribute("target")).toBe("_blank")

    const databaseName = screen.getByText("Shared Database")
    expect(databaseName.closest("a")).toBeNull()
    expect(databaseName.closest("article")).toBeTruthy()
  })

  test("renders independent zero-shared states without hidden placeholders", async () => {
    stubResources(responseWith({
      deployments: {
        summary: { total: 4, shared: 0, hidden: 4 },
        items: [],
      },
      databases: {
        summary: { total: 3, shared: 0, hidden: 3 },
        items: [],
      },
    }))
    render(<GuestPage navigate={vi.fn()} />)

    expect(
      await screen.findByText("No deployments are currently shared."),
    ).toBeTruthy()
    expect(
      screen.getByText("No databases are currently shared."),
    ).toBeTruthy()
    expect(screen.queryByText("Hidden deployment")).toBeNull()
    expect(screen.queryByText("Hidden database")).toBeNull()
  })

  test("isolates an unavailable deployment section", async () => {
    stubResources(responseWith({
      deployments: {
        available: false,
        summary: { total: 0, shared: 0, hidden: 0 },
        items: [],
      },
    }))
    render(<GuestPage navigate={vi.fn()} />)

    expect(
      await screen.findByText(
        "Deployment information is temporarily unavailable.",
      ),
    ).toBeTruthy()
    expect(screen.getByText("Shared Database")).toBeTruthy()
    expect(
      screen.queryByText("Database information is temporarily unavailable."),
    ).toBeNull()
  })

  test("isolates an unavailable database section", async () => {
    stubResources(responseWith({
      databases: {
        available: false,
        summary: { total: 0, shared: 0, hidden: 0 },
        items: [],
      },
    }))
    render(<GuestPage navigate={vi.fn()} />)

    expect(
      await screen.findByText("Database information is temporarily unavailable."),
    ).toBeTruthy()
    expect(screen.getByText("portfolio")).toBeTruthy()
    expect(
      screen.queryByText("Deployment information is temporarily unavailable."),
    ).toBeNull()
  })

  test("shows safe unavailable states when the aggregate request fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
    }))
    render(<GuestPage navigate={vi.fn()} />)

    expect(
      await screen.findByText(
        "Deployment information is temporarily unavailable.",
      ),
    ).toBeTruthy()
    expect(
      screen.getByText("Database information is temporarily unavailable."),
    ).toBeTruthy()
    expect(screen.queryByText("503")).toBeNull()
  })

  test("does not introduce public host or MiniAI telemetry", async () => {
    stubResources(responseWith())
    render(<GuestPage navigate={vi.fn()} />)
    await screen.findByText("portfolio")

    for (const label of [
      "CPU",
      "RAM",
      "Disk",
      "Temperature",
      "Battery",
      "Load average",
      "Uptime",
      "MiniAI",
    ]) {
      expect(screen.queryByText(label, { exact: false })).toBeNull()
    }
  })
})
