import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import AdminShell from "./components/AdminShell"
import ProductNav from "./components/ProductNav"
import { resolveRoute } from "./routing"

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

describe("Observability routing and navigation", () => {
  test("resolves the dedicated Admin route", () => {
    expect(resolveRoute("/admin/observability")).toEqual({
      screen: "observability",
    })
  })

  test("shows Observability in Admin navigation", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ mode: "local" }),
    }))
    render(
      <AdminShell active="observability" navigate={vi.fn()}>
        <div>Page</div>
      </AdminShell>,
    )
    const link = await screen.findByRole("link", { name: "Observability" })
    expect(link.getAttribute("href")).toBe("/admin/observability")
    expect(link.classList.contains("active")).toBe(true)
  })

  test("does not add Observability to the Guest product navigation", () => {
    render(<ProductNav mode="guest" />)
    expect(screen.queryByText("Observability")).toBeNull()
  })
})
