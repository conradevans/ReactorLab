export const SYSTEM_THRESHOLDS = Object.freeze({
  // Load average already smooths short CPU bursts. These normalized thresholds
  // warn near logical-core saturation and turn critical only above 125% host
  // capacity, keeping the Overview signal conservative for this 16-core Dell.
  cpuLoad: Object.freeze({ warning: 0.85, critical: 1.25 }),
  temperature: Object.freeze({ warning: 80, critical: 95 }),
  memory: Object.freeze({ warning: 75, critical: 90 }),
  disk: Object.freeze({ warning: 80, critical: 90 }),
})

function thresholdSeverity(value, thresholds) {
  if (!Number.isFinite(value)) return null
  if (value >= thresholds.critical) return "critical"
  if (value >= thresholds.warning) return "warning"
  return "healthy"
}

export function temperatureSeverity(value) {
  return thresholdSeverity(value, SYSTEM_THRESHOLDS.temperature)
}

export function memorySeverity(value) {
  return thresholdSeverity(value, SYSTEM_THRESHOLDS.memory)
}

export function diskSeverity(value) {
  return thresholdSeverity(value, SYSTEM_THRESHOLDS.disk)
}

export function cpuUsageSeverity() {
  return null
}

export function cpuLoadSeverity(cpu) {
  const logicalCores = Number(cpu?.logicalCores)
  if (!Number.isFinite(logicalCores) || logicalCores <= 0) return null

  const loads = [cpu?.load1, cpu?.load5].filter(Number.isFinite)
  if (loads.length === 0) return null

  return thresholdSeverity(
    Math.max(...loads) / logicalCores,
    SYSTEM_THRESHOLDS.cpuLoad,
  )
}

export function batterySeverity(battery) {
  if (!battery?.available) return null
  return battery.acConnected ? "healthy" : "critical"
}

export function serviceSeverity(service) {
  if (service?.active) return "healthy"

  const transitional = new Set([
    "activating",
    "deactivating",
    "reloading",
    "restarting",
    "maintenance",
  ])
  return transitional.has(String(service?.status || "").toLowerCase())
    ? "warning"
    : "critical"
}
