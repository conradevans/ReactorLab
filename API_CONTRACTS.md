# ReactorLab API Contracts

Status: focused inventory verified November 17, 2026. This is not a generated
or exhaustive route reference. Source handlers and tests remain authoritative.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) is the master narrative.

## Current verified implementation

### ReactorLab to MiniAI

Private, GET-only namespace: `/internal/miniai/v1`.

```text
GET /overview
GET /observability/host
GET /observability/temperature
GET /observability/applications
GET /observability/applications/{id}
GET /observability/services
GET /observability/events
GET /deployments
GET /deployments/{app}
GET /deployments/{app}/history
GET /databases
GET /databases/{id}
GET /databases/{id}/backups
GET /activity
GET /recovery
```

The contract is bounded, typed, and unavailable from public/Guest listeners.
Partial availability is explicit. It excludes credentials, connection strings,
environment values, backup paths, raw logs, arbitrary files, SQL, and shell.
Runtime and build logs remain bounded direct MiniDeploy reads.

### MiniAI administrator actions

```text
POST /api/admin/actions
GET  /api/admin/actions?history_id={history-id}
GET  /api/admin/actions/{action-id}
GET  /api/admin/actions/{action-id}/events
POST /api/admin/actions/{action-id}/confirm
POST /api/admin/actions/{action-id}/cancel
POST /api/admin/actions/{action-id}/execute
```

Confirmation requires `{"confirm":true}`; cancel and execute require empty
bodies. Confirm never executes. Reload/reconciliation use GET only. Only a
persisted, executable, authorized restart proposal can execute, and duplicates
conflict.

The immediate Execute response may include:

```text
execution_outcome = succeeded | failed | outcome_unknown
failure_category  = upstream_unavailable | upstream_rejected |
                    timeout | invalid_response | internal_error
```

Current limitation: normal GET does not reconstruct these transient fields, and
ambiguity has no durable event.

### MiniAI to MiniDeploy restart

```text
POST /deployments/{app}/restart
```

Software binds the application ID. MiniDeploy checks existence and detached
database state before container restart.

- `200`: validated running response for the same application.
- `404`: definitive deployment-not-found rejection.
- `409`: definitive rejection such as a detached database.
- Other status, invalid/read-failed response, or post-dispatch transport
  failure: potentially ambiguous.

MiniAI does not redirect or automatically retry ambiguity.

### MiniDeploy and MiniBase

MiniBase's loopback-only, bearer-authenticated integration includes:

```text
GET    /api/v1/integrations/minideploy/databases
POST   /api/v1/integrations/minideploy/databases
POST   /api/v1/integrations/minideploy/attachments
DELETE /api/v1/integrations/minideploy/attachments/{id}
GET    /api/v1/integrations/minideploy/attachments/{id}/binding
```

The binding route is the sole credential-bearing response and uses no-store
semantics. MiniDeploy separately exposes private deployment listing and
attach/detach lifecycle routes to MiniBase.

### Public and administrator boundaries

- MiniDeploy Guest: `GET /api/guest/deployments`.
- MiniBase Guest: `GET /api/v1/guest/status` and
  `GET /api/v1/guest/databases`.
- ReactorLab Guest aggregation: `GET /api/v1/guest/resources`.
- Administrator namespaces are distinct and Access-protected.
- Guest namespaces do not alias management routes.

## Future roadmap contracts

Not implemented: durable Phase 4D1 reconciliation/outcome APIs, a second
executable action, preview lifecycle, MiniVault, or multi-node contracts.
Future contracts must preserve bounded typed input, least privilege, explicit
authorization, durable audit, and no blind retry.
