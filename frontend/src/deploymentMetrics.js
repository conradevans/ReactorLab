export function deploymentStatusLabel(status) {
  if (status === "healthy") return "Healthy"
  if (status === "degraded") return "Degraded"
  if (status === "database-detached") return "Database detached"
  if (status === "unavailable") return "Unavailable"
  return status || "Unknown"
}

export function deploymentStatusClass(status) {
  if (status === "healthy") return "deployment-healthy"
  if (status === "degraded" || status === "database-detached") {
    return "deployment-warning"
  }
  return "deployment-error"
}

export function summarizeDeployment(deployment) {
  const containers = deployment?.containers ?? []

  return containers.reduce(
    (summary, container) => ({
      containers: summary.containers + 1,
      cpuPercent: summary.cpuPercent + (container.cpuPercent || 0),
      memoryUsedBytes:
        summary.memoryUsedBytes + (container.memoryUsedBytes || 0),
      memoryPercent:
        summary.memoryPercent + (container.memoryPercent || 0),
      networkRXBytes:
        summary.networkRXBytes + (container.networkRxBytes || 0),
      networkTXBytes:
        summary.networkTXBytes + (container.networkTxBytes || 0),
      writableBytes: summary.writableBytes + (container.writableBytes || 0),
      restarts: summary.restarts + (container.restartCount || 0),
      pids: summary.pids + (container.pids || 0),
    }),
    {
      containers: 0,
      cpuPercent: 0,
      memoryUsedBytes: 0,
      memoryPercent: 0,
      networkRXBytes: 0,
      networkTXBytes: 0,
      writableBytes: 0,
      restarts: 0,
      pids: 0,
    },
  )
}
