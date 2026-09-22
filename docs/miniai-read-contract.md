# MiniAI read contract

ReactorLab exposes a versioned, read-only service contract for MiniAI 2.0 at
`/internal/miniai/v1`. The routes are registered only on ReactorLab's private
loopback listener. They are not available from `PublicHandler`, Guest APIs, or
the public listener.

## Endpoints

- `GET /internal/miniai/v1/overview`
- `GET /internal/miniai/v1/observability/host`
- `GET /internal/miniai/v1/observability/temperature`
- `GET /internal/miniai/v1/observability/applications`
- `GET /internal/miniai/v1/observability/applications/{id}`
- `GET /internal/miniai/v1/observability/services`
- `GET /internal/miniai/v1/observability/events`
- `GET /internal/miniai/v1/deployments`
- `GET /internal/miniai/v1/deployments/{app}`
- `GET /internal/miniai/v1/deployments/{app}/history`
- `GET /internal/miniai/v1/databases`
- `GET /internal/miniai/v1/databases/{id}`
- `GET /internal/miniai/v1/databases/{id}/backups`
- `GET /internal/miniai/v1/activity`
- `GET /internal/miniai/v1/recovery`

There are no mutation routes. Runtime and deployment-log reads are deliberately
deferred: safely bounding already-redacted multi-service output would require a
larger proxy and truncation contract than Phase 1B.

## Historical windows and bounds

Observability endpoints accept either `range=15m|1h|6h|24h|7d` or an explicit
`from=<RFC3339>&to=<RFC3339>` pair. If no window is supplied, `1h` is used.
Explicit timestamps are normalized to UTC, must be ordered, cannot extend into
the future or before the retained seven-day metric horizon, and cannot span
more than seven days.

ReactorLab chooses a deterministic bucket that permits no more than 240 metric
points per series. Event reads default to 100 and are capped at 500. Activity
and backup reads default to 100 and are capped at 200. Events, Activity, and
backups are returned newest first.

## Partial availability

The overview is assembled from independent current-system, recovery,
MiniDeploy, MiniBase, and historical-observability sources. It always returns
typed availability sections. Failure of one source marks only its section
unavailable with a stable error code; raw upstream errors are never returned.
Individual endpoints return stable machine-readable 4xx or 503 error codes.

## Sources of truth

The contract creates no persistence or cache:

- current host and recovery state come from `internal/system`;
- historical host, temperature, application, service, event, and recovery data
  come through `observability.QueryService`;
- current deployment provenance and rollback history come from MiniDeploy's
  private management API;
- current database metrics and backup inventory come from MiniBase;
- recent platform/database Activity comes from ReactorLab's existing Activity
  store.

Legacy deployments may have no source, activation time, or immutable image ID.
Those fields remain absent. ReactorLab never infers provenance from a checkout,
filesystem state, image name, or repository URL. Historical `archivedAt`
identifies when MiniDeploy archived a version and remains distinct from its
original `activatedAt`.

## Security boundary

All routes are GET-only and use bounded identifiers, time windows, limits,
response sizes, and request timeouts. Upstream JSON is treated as untrusted and
validated before projection. The contract does not expose raw repository URLs,
environment values, database credentials or connection strings, backup paths,
recovery boot IDs, Activity fingerprints, arbitrary event details, filesystem
reads, SQL, shell commands, or root actions.
