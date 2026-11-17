# ReactorLab Roadmap

Status: verified roadmap position as of November 17, 2026.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) is the master narrative.
Listing a phase here does not authorize implementation.

## Current verified implementation

Completed foundation:

- MiniDeploy deployment/runtime, restart, rollback, history, visibility,
  full-stack lifecycle, database binding, observability, and exact source identity.
- MiniBase PostgreSQL provisioning, backup/restore, retention, deletion safety,
  attachments, Guest visibility, Access-protected public origin, explorer, and Activity.
- ReactorLab unified views, historical observability, host recovery, Guest
  aggregation, and private MiniAI read contract.
- MiniAI Phase 0 architecture foundation.
- Phase 1 typed read evidence.
- Phase 2 bounded investigation and one-pass reasoner.
- Phase 3 durable History.
- Phase 4A policy, durable actions, auth, and action API.
- Phase 4B narrow restart executor and controlled execution.
- Phase 4C assistant suggestions, semantic eligibility, action UI, reload
  safety, and live canary proof.

```text
Phase 4C2
Interactive confirmation/execution UI
COMPLETE + LIVE PROVEN
```

## Exact next milestone

```text
Phase 4D1
Action failure, ambiguity, reconciliation, and recovery hardening
NOT STARTED
```

Primary repository: MiniAI. Dependencies: MiniDeploy restart and ReactorLab
current-state evidence. Fixture: miniai-restart-canary. MiniBase is outside the
milestone.

Acceptance intent:

- definitive pre-mutation rejection becomes a durable failed outcome;
- ambiguity remains explicitly unknown;
- reconciliation is GET-only and never retries mutation;
- current state is not mislabeled as causal proof;
- reload remains read-only;
- duplicate execution cannot duplicate restart;
- safe outcome, category, timing, and reconciliation facts are auditable;
- bounded canary proof validates unhappy paths.

The current code has groundwork but does not restore ambiguity or failure
details through normal GET/reload.

## Future roadmap

| Phase | Planned outcome | Status |
|---|---|---|
| 4D2 | Action observability and recovery UX | Future |
| 4D3 | Second narrow typed action | Future |
| 5 | Preview deployments | Future |
| 6 | Git-backed controlled source editing | Future |
| 7 | Controlled web research | Future |
| 8 | AI/action evaluation and hardening | Future |
| 9 | Log Explorer and observability | Future |
| 10 | Self-healing through typed actions | Future |
| 11 | MiniBase object/file storage | Future |
| 12 | MiniVault | Future |
| 13 | Least-privilege specialized agents | Future |
| 14 | Platform backup/disaster recovery | Future |
| 15 | Installer, hardware detection, security hardening | Future |
| 16 | Deployment templates | Future |
| 17 | Open-source productization | Future |
| 18 | Minimal secure multi-node ReactorLab | Future |

## Audit follow-ups that do not advance the roadmap

- Align ReactorLab's README with the existing MiniAI read contract.
- Refresh stale MiniDeploy and MiniBase roadmap/architecture text.
- Make ReactorLab Linux collector tests injectable/runnable on the Mac.
- Fix MiniDeploy's macOS temporary-path assertion.
- Add a Darwin-compatible MiniBase build boundary or explicitly require Linux.
- Add a current MiniAI architecture/roadmap entry point.

These require separate authorization if implementation is involved.

## Authorization rule

The work that created this file was authorized only for:

```text
AUDIT / DOCUMENTATION ONLY
```

No Phase 4D1 implementation, production access, deployment, or live mutation
was authorized or performed. Documentation completion grants no later authority.
