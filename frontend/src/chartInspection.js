export const CHART_GEOMETRY = Object.freeze({
  width: 680,
  height: 190,
  top: 16,
  bottom: 28,
  left: 12,
  right: 12,
})

function clamp(value, minimum, maximum) {
  return Math.min(Math.max(value, minimum), maximum)
}

export function chartPointTimestamp(point) {
  const timestamp = new Date(point?.timestamp || point?.bucketStart).getTime()
  return Number.isFinite(timestamp) ? timestamp : null
}

export function prepareChartPoints(points) {
  return points
    .map((point, originalIndex) => ({
      point,
      originalIndex,
      timestamp: chartPointTimestamp(point),
    }))
    .filter((entry) => entry.timestamp !== null)
    .sort((left, right) => (
      left.timestamp - right.timestamp || left.originalIndex - right.originalIndex
    ))
}

export function weightedRangeAverage(points, valueForPoint) {
  let weightedTotal = 0
  let totalSamples = 0
  for (const point of points) {
    const value = valueForPoint(point)
    const sampleCount = point?.sampleCount
    if (!Number.isFinite(value) || !Number.isFinite(sampleCount) || sampleCount <= 0) continue
    weightedTotal += value * sampleCount
    totalSamples += sampleCount
  }
  if (totalSamples === 0) return null
  const average = weightedTotal / totalSamples
  return Number.isFinite(average) ? average : null
}

export function rangeMaximum(points, valueForPoint) {
  let maximum = null
  for (const point of points) {
    const value = valueForPoint(point)
    if (!Number.isFinite(value)) continue
    maximum = maximum === null ? value : Math.max(maximum, value)
  }
  return maximum
}

export function summarizeChartSeries(points, series) {
  return series.map((item) => ({
    ...item,
    summaryValue: item.summaryAggregation === "max"
      ? rangeMaximum(points, item.value)
      : item.summaryAggregation === "average"
        ? weightedRangeAverage(points, item.value)
        : null,
  }))
}

export function inspectChartPosition(clientX, rect, start, end, geometry = CHART_GEOMETRY) {
  const rawX = rect.width > 0
    ? ((clientX - rect.left) * geometry.width) / rect.width
    : geometry.left
  const plotEnd = geometry.width - geometry.right
  const x = clamp(rawX, geometry.left, plotEnd)
  const normalized = clamp(
    (x - geometry.left) / (plotEnd - geometry.left),
    0,
    1,
  )
  return {
    normalized,
    timestamp: start + normalized * Math.max(end - start, 0),
    x,
  }
}

function finiteValue(entry, valueForPoint) {
  const value = valueForPoint(entry.point)
  return Number.isFinite(value) ? value : null
}

function exactValue(entries, startIndex, valueForPoint) {
  const timestamp = entries[startIndex].timestamp
  for (let index = startIndex; index < entries.length; index += 1) {
    if (entries[index].timestamp !== timestamp) break
    const value = finiteValue(entries[index], valueForPoint)
    if (value !== null) return value
  }
  return null
}

export function interpolateChartValue(entries, valueForPoint, cursorTimestamp) {
  if (entries.length === 0 || !Number.isFinite(cursorTimestamp)) return null

  let low = 0
  let high = entries.length
  while (low < high) {
    const middle = Math.floor((low + high) / 2)
    if (entries[middle].timestamp < cursorTimestamp) low = middle + 1
    else high = middle
  }

  if (low < entries.length && entries[low].timestamp === cursorTimestamp) {
    return exactValue(entries, low, valueForPoint)
  }
  if (low === 0) return exactValue(entries, 0, valueForPoint)
  if (low === entries.length) {
    let lastTimestampIndex = entries.length - 1
    while (
      lastTimestampIndex > 0 &&
      entries[lastTimestampIndex - 1].timestamp === entries[lastTimestampIndex].timestamp
    ) {
      lastTimestampIndex -= 1
    }
    return exactValue(entries, lastTimestampIndex, valueForPoint)
  }

  const left = entries[low - 1]
  const right = entries[low]
  const leftValue = finiteValue(left, valueForPoint)
  const rightValue = finiteValue(right, valueForPoint)
  if (leftValue === null || rightValue === null) return null

  const duration = right.timestamp - left.timestamp
  if (duration <= 0) return leftValue
  const ratio = (cursorTimestamp - left.timestamp) / duration
  return leftValue + ratio * (rightValue - leftValue)
}

export function formatInspectionTimestamp(value, locales) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"
  return date.toLocaleString(locales, {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  })
}
