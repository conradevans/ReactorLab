import { describe, expect, it } from "vitest"

import {
  deploymentStatusClass,
  deploymentStatusLabel,
  summarizeDeployment,
} from "./deploymentMetrics"

describe("deployment metrics", () => {
  it("summarizes all containers in a deployment", () => {
    const summary = summarizeDeployment({
      containers: [
        {
          cpuPercent: 0.2,
          memoryUsedBytes: 100,
          memoryPercent: 0.5,
          networkRxBytes: 10,
          networkTxBytes: 20,
          writableBytes: 30,
          restartCount: 2,
          pids: 4,
        },
        {
          cpuPercent: 0.3,
          memoryUsedBytes: 200,
          memoryPercent: 0.7,
          networkRxBytes: 40,
          networkTxBytes: 50,
          writableBytes: 60,
          restartCount: 1,
          pids: 5,
        },
      ],
    })

    expect(summary).toEqual({
      containers: 2,
      cpuPercent: 0.5,
      memoryUsedBytes: 300,
      memoryPercent: 1.2,
      networkRXBytes: 50,
      networkTXBytes: 70,
      writableBytes: 90,
      restarts: 3,
      pids: 9,
    })
  })

  it("maps deployment states to stable UI labels and classes", () => {
    expect(deploymentStatusLabel("database-detached")).toBe("Database detached")
    expect(deploymentStatusClass("healthy")).toBe("deployment-healthy")
    expect(deploymentStatusClass("database-detached")).toBe("deployment-warning")
    expect(deploymentStatusClass("unavailable")).toBe("deployment-error")
  })
})
