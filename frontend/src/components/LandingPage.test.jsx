import { cleanup, render, screen } from "@testing-library/react"
import { afterEach, describe, expect, test, vi } from "vitest"

import LandingPage from "./LandingPage"

afterEach(cleanup)

describe("ReactorLab access page", () => {
  test("clearly explains the product and preserves both access paths", () => {
    render(<LandingPage navigate={vi.fn()} />)

    expect(screen.getByText("What it does")).toBeTruthy()
    expect(screen.getByText("How it works")).toBeTruthy()
    expect(screen.getByText("Why it is useful")).toBeTruthy()
    expect(
      screen.getByText(/Administrator access opens detailed infrastructure/),
    ).toBeTruthy()
    expect(
      screen.queryByText(/Administrator access is protected by Cloudflare Access/),
    ).toBeNull()
    expect(
      screen.getByRole("link", { name: "Open Administrator" }).getAttribute("href"),
    ).toBe("/admin")
    expect(
      screen.getByRole("link", { name: "Guest Overview" }).getAttribute("href"),
    ).toBe("/guest")
    expect(screen.queryByText("MiniAI")).toBeNull()
  })
})
