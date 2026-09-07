export function activityKindLabel(kind) {
  if (kind === "warning") return "Warning"
  if (kind === "recovery") return "Recovered"
  return kind ? kind.charAt(0).toUpperCase() + kind.slice(1) : "Event"
}

export function activitySeverityClass(severity) {
  if (severity === "warning") return "activity-severity-warning"
  if (severity === "error") return "activity-severity-error"
  if (severity === "info") return "activity-severity-info"
  return "activity-severity-neutral"
}

export function activitySourceLabel(source) {
  if (source === "minideploy") return "MiniDeploy"
  if (source === "minibase") return "MiniBase"
  if (source === "system") return "Dell"
  if (source === "relationship") return "Relationship"
  if (source === "reactorlab") return "ReactorLab"
  return source || "ReactorLab"
}

export function formatActivityTime(value) {
  if (!value) return "—"

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"

  return date.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  })
}
