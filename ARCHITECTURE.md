# ReactorLab Architecture

Status: verified workspace architecture as of November 17, 2026.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) is the master narrative and
engineering handoff. This document is the focused architecture reference.
Repositories and Git history remain authoritative when newer.

## Current verified implementation

ReactorLab is currently a single-node personal developer cloud:

```text
                         ReactorLab
                  platform view and evidence
                    /          |          \
          MiniDeploy        MiniBase       host/recovery
        application runtime   data plane     evidence
                    \         /
                       MiniAI
              investigation and typed actions
                         |
             miniai-restart-canary
                bounded mutation fixture
```

| Component | Authoritative responsibility |
|---|---|
| ReactorLab | Unified platform state, historical observability, host recovery evidence, relationships, and the private MiniAI read contract |
| MiniDeploy | Source identity, builds, deployments, containers, health, logs, restart, redeploy, rollback, routing, and deployment history |
| MiniBase | PostgreSQL databases and roles, credentials, attachments, backups, restore, deletion safety, exploration, and Activity |
| MiniAI | Typed evidence, bounded reasoning, History, action policy, authorization, audit records, and the narrow restart executor |
| miniai-restart-canary | Disposable application for restart proof without risking real workloads |

### Authoritative flows

MiniAI reads platform state through ReactorLab's loopback-only
`/internal/miniai/v1` contract. ReactorLab combines its own state with
validated MiniDeploy and MiniBase responses. Runtime and deployment logs remain
a deliberate exception read directly from MiniDeploy.

MiniAI sends `restart_application` through
`POST /deployments/{app}/restart`. It does not operate Docker or systemd.
MiniDeploy remains authoritative for application lifecycle.

MiniDeploy and MiniBase use private authenticated integration contracts.
MiniBase owns the database and credential; MiniDeploy owns the application
binding and runtime injection. Code rollback and database rollback are separate
timelines.

### Current MiniAI actions

```text
awaiting_confirmation -> authorized -> executing -> succeeded
                    \              \-> failed
                     \-> cancelled
```

Assistant-suggested restart actions require confirmation. Confirmation and
execution are separate HTTP mutations. Only `restart_application` has a real
executor; catalog entries for rollback and deploy are not executable.

The executor persists `executing` before calling MiniDeploy and never retries
after dispatch. Definitive pre-mutation rejection can become `failed`.
Transport or response ambiguity leaves the action `executing` and returns a
transient `outcome_unknown`.

### Current persistence and deployment

- ReactorLab stores Activity and historical observability in SQLite.
- MiniDeploy stores deployment metadata, exact source identity, history, and logs.
- MiniBase stores non-secret metadata in SQLite and application data in a
  private PostgreSQL service with per-project databases and roles.
- MiniAI stores chats, History, proposals, and events in SQLite schema version 3.
- Passwords and other secrets are not stored in ordinary metadata records.

The Dell is the production node. Services bind to loopback origins; public
traffic uses Cloudflare Tunnel and Cloudflare Access protects administrator
surfaces with origin-side validation. PostgreSQL has no public host port.

There is no current multi-node scheduler, consensus layer, or service mesh.

## Known current limitations

- Ambiguous MiniAI outcomes are not durable or recoverable through normal GET
  after reload.
- Normal action GET responses omit the failure category stored on failure events.
- The action UI renders generic executing and failed states.
- MiniDeploy restart has no durable operation identity proving causation.
- Same-Dell backups do not protect against physical storage loss.
- Some Linux-specific code/tests do not run cleanly on the Mac; see
  [`TESTING.md`](TESTING.md).

## Future roadmap architecture

Planned, not current: Phase 4D1 durable failure/ambiguity and reconciliation;
4D2 recovery UX; 4D3 a second action; previews; Git-backed source editing;
controlled web research; evaluation; expanded observability; self-healing;
MiniBase object storage; MiniVault; specialized least-privilege agents;
off-node disaster recovery; installer/productization; and late multi-node
ReactorLab.

Future capabilities require explicit authorization and must not be described as
implemented.
