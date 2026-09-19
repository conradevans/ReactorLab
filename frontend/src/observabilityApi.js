import { getJSON } from "./api"

export const OBSERVABILITY_RANGES = ["15m", "1h", "6h", "24h", "7d"]

function checkedRange(range) {
  if (!OBSERVABILITY_RANGES.includes(range)) {
    throw new Error("invalid observability range")
  }
  return encodeURIComponent(range)
}

export async function getObservabilityOverview(range, options = {}) {
  const safeRange = checkedRange(range)
  const root = "/api/v1/observability"
  const [host, temperature, apps, services, events] = await Promise.all([
    getJSON(`${root}/host?range=${safeRange}`, options),
    getJSON(`${root}/temperature?range=${safeRange}`, options),
    getJSON(`${root}/apps?range=${safeRange}`, options),
    getJSON(`${root}/services?range=${safeRange}`, options),
    getJSON(`${root}/events?range=${safeRange}`, options),
  ])
  return {
    host: host.data?.points || [],
    temperature: temperature.data?.points || [],
    applications: apps.data?.applications || [],
    services: services.data?.services || [],
    events: events.data?.events || [],
    from: host.from,
    to: host.to,
  }
}

export async function getApplicationObservability(appID, range, options = {}) {
  if (!appID) return []
  const safeRange = checkedRange(range)
  const response = await getJSON(
    `/api/v1/observability/apps/${encodeURIComponent(appID)}?range=${safeRange}`,
    options,
  )
  return response.data?.points || []
}
