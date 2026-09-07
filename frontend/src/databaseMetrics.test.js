import { describe, expect, it } from "vitest"

import {
  cacheHitPercent,
  databaseStatusClass,
  databaseStatusLabel,
  formatBackupAge,
  summarizeDatabases,
} from "./databaseMetrics"

describe("database metrics helpers", () => {
  it("formats database states and backup age", () => {
    expect(databaseStatusLabel("ready")).toBe("Ready")
    expect(databaseStatusLabel("metadata_only")).toBe("Metadata only")
    expect(databaseStatusClass("ready")).toBe("status-ready")
    expect(databaseStatusClass("error")).toBe("status-error")
    expect(formatBackupAge(3660)).toBe("1h 1m ago")
    expect(formatBackupAge(null)).toBe("No completed backup")
  })

  it("summarizes database storage, connections, and backups", () => {
    expect(
      summarizeDatabases([
        {
          sizeBytes: 100,
          connections: 2,
          backupCount: 3,
          backupBytes: 50,
        },
        {
          sizeBytes: 200,
          connections: 1,
          backupCount: 1,
          backupBytes: 25,
        },
      ]),
    ).toEqual({
      sizeBytes: 300,
      connections: 3,
      backups: 4,
      backupBytes: 75,
    })
  })

  it("calculates cache hit percentage without inventing an empty rate", () => {
    expect(
      cacheHitPercent({
        cache: {
          blockReads: 25,
          blockHits: 75,
        },
      }),
    ).toBe(75)
    expect(cacheHitPercent({ cache: { blockReads: 0, blockHits: 0 } })).toBeNull()
  })
})
