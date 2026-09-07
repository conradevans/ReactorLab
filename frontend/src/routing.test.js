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

  it("resolves database list and detail routes", () => {
    expect(resolveRoute("/admin/databases")).toEqual({
      screen: "databases",
    })
    expect(
      resolveRoute("/admin/databases/database_0123456789abcdef0123456789abcdef"),
    ).toEqual({
      screen: "database-detail",
      id: "database_0123456789abcdef0123456789abcdef",
    })
  })

  it("rejects nested or malformed database detail paths", () => {
    expect(resolveRoute("/admin/databases/a/b")).toEqual({
      screen: "not-found",
    })
    expect(resolveRoute("/admin/databases/%E0%A4%A")).toEqual({
      screen: "not-found",
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
