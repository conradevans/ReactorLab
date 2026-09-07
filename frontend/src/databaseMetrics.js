import { formatUptime } from "./format"

export function databaseStatusLabel(status) {
  if (status === "ready") return "Ready"
  if (status === "provisioning") return "Provisioning"
  if (status === "metadata_only") return "Metadata only"
  if (status === "error") return "Error"
  return status || "Unknown"
}

export function databaseStatusClass(status) {
  if (status === "ready") return "status-ready"
  if (status === "provisioning") return "status-provisioning"
  if (status === "metadata_only") return "status-metadata_only"
  return "status-error"
}

export function formatBackupAge(value) {
  return Number.isFinite(value)
    ? `${formatUptime(value)} ago`
    : "No completed backup"
}

export function cacheHitPercent(database) {
  const reads = database?.cache?.blockReads ?? 0
  const hits = database?.cache?.blockHits ?? 0
  const total = reads + hits
  return total > 0 ? (hits / total) * 100 : null
}

export function summarizeDatabases(databases = []) {
  return databases.reduce(
    (summary, database) => ({
      sizeBytes: summary.sizeBytes + (database.sizeBytes || 0),
      connections: summary.connections + (database.connections || 0),
      backups: summary.backups + (database.backupCount || 0),
      backupBytes: summary.backupBytes + (database.backupBytes || 0),
    }),
    {
      sizeBytes: 0,
      connections: 0,
      backups: 0,
      backupBytes: 0,
    },
  )
}
