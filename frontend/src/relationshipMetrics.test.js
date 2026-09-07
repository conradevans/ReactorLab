import { describe, expect, it } from "vitest"
import {
  relationshipStateClass,
  relationshipStateLabel,
} from "./relationshipMetrics"

describe("relationshipStateLabel", () => {
  it("formats known and unknown relationship states", () => {
    expect(relationshipStateLabel("linked")).toBe("Linked")
    expect(relationshipStateLabel("detached")).toBe("Detached")
    expect(relationshipStateLabel("unavailable")).toBe("Unavailable")
    expect(relationshipStateLabel("unresolved")).toBe("Unresolved")
    expect(relationshipStateLabel("conflict")).toBe("Conflict")
    expect(relationshipStateLabel()).toBe("Unknown")
  })
})

describe("relationshipStateClass", () => {
  it("maps relationship states to visual classes", () => {
    expect(relationshipStateClass("linked")).toBe("relationship-linked")
    expect(relationshipStateClass("detached")).toBe("relationship-detached")
    expect(relationshipStateClass("unresolved")).toBe("relationship-unresolved")
    expect(relationshipStateClass("unavailable")).toBe("relationship-unavailable")
    expect(relationshipStateClass("conflict")).toBe("relationship-conflict")
    expect(relationshipStateClass()).toBe("relationship-unknown")
  })
})
