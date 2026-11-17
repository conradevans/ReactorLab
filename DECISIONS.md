# ReactorLab Architecture Decisions

Status: consolidated decisions verified November 17, 2026. These are concise
records; [`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) contains the full
rationale and history.

## Current decisions

### ADR-001: MiniAI has no arbitrary shell capability

Accepted and implemented. MiniAI uses closed typed reads and narrow executors.
A generic shell would collapse service ownership and authorization.

### ADR-002: Application lifecycle mutation belongs to MiniDeploy

Accepted and implemented. MiniAI requests restart from MiniDeploy and does not
manipulate containers directly.

### ADR-003: Managed application data belongs to MiniBase

Accepted and implemented. MiniBase owns databases, roles, credentials,
attachments, backup, restore, and deletion lifecycle. Code and data history are
separate.

### ADR-005: Model output is not authoritative infrastructure identity

Accepted and implemented. A model may suggest intent; deterministic software
resolves and validates the real target.

### ADR-006: Confirmation and execution are separate transitions

Accepted, implemented, and live-proven. Confirm records authorization. A
separate request executes. Reload performs neither.

### ADR-007: Ambiguous mutations are not automatically retried

Accepted and implemented for restart dispatch. The deployed Phase 4D1
implementation adds an explicit durable ambiguity event and GET-only
reconciliation. Unknown outcomes remain unknown without authoritative causal
proof. Automated ambiguity coverage passes; live ambiguity was intentionally
not induced. Overall phase acceptance remains partial for a separate live
assistant-suggestion limitation.

### ADR-008: Current evidence is required for restart eligibility

Accepted, implemented, and live-proven. Historical errors alone do not justify
restart; trusted current evidence must establish the closed condition.

### ADR-009: History and action audit records are durable production evidence

Accepted and implemented. History, proposals, events, failures, and bug
fixtures are not disposable test data.

### ADR-013: Single-node maturity precedes multi-node expansion

Accepted. ReactorLab remains single-node until its deployment, data, security,
recovery, and AI-operation foundations are reference-quality.

## Future-sequencing decisions

These are accepted direction, not implemented capability:

### ADR-004: Secrets and service identity belong to MiniVault

MiniVault is the selected future authority for encrypted secrets, credentials,
service identity, bindings, rotation, revocation, and audit. It is not a current
repository or implemented service.

### ADR-010: Source editing is Git-backed

Future source changes use branches, diffs, tests, previews, approval, and
merge/deployment. Production checkouts are not edited directly.

### ADR-011: Preview environments precede source editing

MiniAI must have an isolated verification target before receiving controlled
source-write capability.

### ADR-012: Agents use least privilege

Future specialized agents receive distinct typed capabilities and identities,
not one omnipotent tool set or merely different prompts.

### ADR-014: Multi-node remains part of the final ReactorLab vision

Multi-node is deliberately late and must remain substantially simpler than
Kubernetes, introducing only complexity required by real ReactorLab workloads.

## Decision priority

```text
1. Safety
2. Correctness
3. Auditability
4. Recoverability
5. Clear component ownership
6. Simplicity
7. Developer experience
8. Automation
9. Performance
```

Changing a hard trust-boundary decision requires explicit architecture review,
not an incidental implementation patch.
