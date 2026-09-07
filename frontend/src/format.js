export function formatPercent(value) {
  return Number.isFinite(value) ? `${value.toFixed(1)}%` : "—"
}

export function formatTemperature(value) {
  return Number.isFinite(value) ? `${value.toFixed(1)}°C` : "—"
}

export function formatBytes(value) {
  if (!Number.isFinite(value)) return "—"

  const units = ["B", "KiB", "MiB", "GiB", "TiB"]
  let amount = value
  let index = 0

  while (amount >= 1024 && index < units.length - 1) {
    amount /= 1024
    index += 1
  }

  const digits = index === 0 || amount >= 100 ? 0 : 1
  return `${amount.toFixed(digits)} ${units[index]}`
}

export function formatRate(value) {
  if (!Number.isFinite(value)) return "—"
  return `${formatBytes(value)}/s`
}

export function formatUptime(value) {
  if (!Number.isFinite(value)) return "—"

  const totalMinutes = Math.floor(value / 60)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60

  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}
