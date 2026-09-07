import { describe, expect, it } from "vitest"

import { resolveRoute } from "./routing"

describe("resolveRoute", () => {
  it("resolves deployment list and detail routes", () => {
    expect(resolveRoute("/admin/deployments")).toEqual({
      screen: "deployments",
    })
    expect(resolveRoute("/admin/deployments/golfmullet")).toEqual({
      screen: "deployment-detail",
      app: "golfmullet",
    })
  })

  it("rejects nested or malformed deployment detail paths", () => {
    expect(resolveRoute("/admin/deployments/a/b")).toEqual({
      screen: "not-found",
    })
    expect(resolveRoute("/admin/deployments/%E0%A4%A")).toEqual({
      screen: "not-found",
    })
  })
})
