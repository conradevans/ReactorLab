export function relationshipStateLabel(state) {
  if (state === "linked") return "Linked"
  if (state === "detached") return "Detached"
  if (state === "unavailable") return "Unavailable"
  if (state === "unresolved") return "Unresolved"
  if (state === "conflict") return "Conflict"
  return "Unknown"
}

export function relationshipStateClass(state) {
  if (state === "linked") return "relationship-linked"
  if (state === "detached") return "relationship-detached"
  if (state === "unresolved") return "relationship-unresolved"
  if (state === "unavailable") return "relationship-unavailable"
  if (state === "conflict") return "relationship-conflict"
  return "relationship-unknown"
}
