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

export function formatDuration(value) {
  if (!Number.isFinite(value) || value < 0) return "—"

  const totalSeconds = Math.floor(value)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60

  if (hours > 0) return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
  if (minutes > 0) return seconds > 0 ? `${minutes}m ${seconds}s` : `${minutes}m`
  return `${seconds}s`
}

export function formatCompactDate(value) {
  if (!value) return "—"

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"

  return date.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  })
}
