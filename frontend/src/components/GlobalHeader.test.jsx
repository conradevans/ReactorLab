import { cleanup, fireEvent, render, screen, within } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import GlobalHeader from "./GlobalHeader"

afterEach(cleanup)

function productNames(container) {
  return within(
    within(container).getByRole("navigation", {
      name: "ReactorLab products",
    }),
  )
    .getAllByRole("link")
    .map((link) => link.textContent)
}

describe("ReactorLab global header", () => {
  test.each([
    ["root", ["ReactorLab", "MiniDeploy", "MiniBase"]],
    ["guest", ["ReactorLab", "MiniDeploy", "MiniBase"]],
    ["admin", ["ReactorLab", "MiniDeploy", "MiniBase", "MiniAI"]],
  ])("shows the correct %s product navigation", (mode, expected) => {
    const view = render(
      <GlobalHeader
        mode={mode}
        navigate={vi.fn()}
        sessionLabel={mode === "guest" ? "Guest View" : mode === "admin" ? "Admin" : ""}
      />,
    )
    expect(productNames(view.container)).toEqual(expected)
    expect(
      screen.getByRole("link", { name: "ReactorLab home" }).textContent,
    ).toContain("ReactorLab")
    expect(
      screen.getByRole("link", { name: "ReactorLab" }).getAttribute("aria-current"),
    ).toBe("page")
  })

  test("shows the administrator identity in the shared Access session card", () => {
    render(
      <GlobalHeader
        mode="admin"
        navigate={vi.fn()}
        sessionLabel="Admin · user@example.com"
      />,
    )
    expect(screen.getByText("ACCESS SESSION").tagName).toBe("SMALL")
    expect(screen.getByText("user@example.com").tagName).toBe("STRONG")
    expect(
      screen.getByLabelText("Access session: user@example.com"),
    ).toBeTruthy()
  })

  test("uses the safe Administrator fallback without inventing an email", () => {
    render(
      <GlobalHeader mode="admin" navigate={vi.fn()} sessionLabel="Admin" />,
    )
    expect(screen.getByText("Administrator")).toBeTruthy()
  })

  test("mobile menu closes on Escape and switches current-product access", () => {
    const navigate = vi.fn()
    render(
      <GlobalHeader
        mode="guest"
        navigate={navigate}
        sessionLabel="Guest View"
      />,
    )

    fireEvent.click(screen.getByRole("button", { name: "Open product menu" }))
    expect(screen.getAllByText("Guest View")).toHaveLength(2)
    fireEvent.keyDown(document, { key: "Escape" })
    expect(screen.queryByRole("button", { name: "Close product menu" })).toBeNull()

    fireEvent.click(screen.getByRole("button", { name: "Open product menu" }))
    const switches = screen.getAllByRole("link", { name: "Switch Access" })
    fireEvent.click(switches[switches.length - 1])
    expect(navigate).toHaveBeenCalledWith("/")
  })
})
