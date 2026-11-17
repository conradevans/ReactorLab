# ReactorLab Data Models

Status: verified conceptual ownership as of November 17, 2026. Source migrations
and tests are authoritative.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) remains the master narrative.

## Current verified implementation

### ReactorLab

ReactorLab owns platform Activity and a separate SQLite historical-observability
store. Observability contains bounded host, temperature, application, and
service samples plus infrastructure and recovery events. Metric retention is
seven days; durable events are not ordinary metric samples.

Cross-service records are validated and projected. Secret-bearing upstream
records are not copied into public contracts.

### MiniDeploy

Deployment data includes canonical application identity, strategy and service
topology, runtime/container identity, requested ref, exact commit SHA when
available, canonical repository identity, activation time, immutable image
identity, database attachment metadata, Guest visibility, and deployment
history.

`activatedAt` is when a version activated; `archivedAt` is when it later
entered history. Runtime environment values are separate from public metadata.
Database contents are not deployment rollback data.

### MiniBase

The current SQLite metadata schema is version 6. It stores non-secret records
for databases, backups, MiniDeploy attachments, Activity, and Guest visibility.
Application data lives in PostgreSQL 17. Each managed project receives a
dedicated database, role, and filesystem credential.

Passwords, connection strings, and archive paths are not ordinary metadata.
Deletion is a serialized lifecycle: attached databases are blocked, external
resources are removed before metadata, and partial cleanup remains retryable.

Backup records identify database, kind, lifecycle state, size, and timestamps.
Archive content is separate. Replace-current restore first creates a retained
pre-restore safety backup. Attachment records contain non-secret database,
consumer, and binding identity.

### MiniAI History and actions

History stores request/result identity and operational facts such as route,
model, confidence, reads, reasoner calls, validation, token/timing facts, and
errors. It is durable production evidence.

MiniAI's deployed SQLite schema version is 4. The November 17, 2026 migration
preserved all pre-existing proposal, event, and History row hashes: 5 proposals,
15 events, and 23 History entries at deployment. These are historical baseline
counts, not permanent totals; subsequent acceptance creates new records.
Action proposals persist:

- action ID, kind, origin, target, and typed arguments;
- safety and policy decisions;
- confirmation/warning requirements;
- the execution-support snapshot;
- current state and optional History link;
- created and updated timestamps.

Current states:

```text
awaiting_confirmation
authorized
executing
succeeded
failed
cancelled
```

Current events:

```text
proposal_created
policy_authorized_on_creation
confirmation_granted
cancelled
execution_started
execution_succeeded
execution_failed
execution_outcome_unknown
```

Failure events store one closed, safe category. Raw upstream errors, local paths,
credentials, and provider bodies are not action data.

Schema v4 adds an ambiguity event with `executing` as both source and
destination state. Migration preserves all prior proposal and event values.
Schema v3's event CHECK constraints require this small migration to represent
ambiguity explicitly without falsely labeling it failure.

Outcomes, categories, and timestamps are derived from validated audit chains.
An interrupted legacy `executing` action projects as unknown without an invented
category or completion time. Refresh reads this projection and writes no events.
The old v3 binary cannot open v4: deployment must preserve a consistent backup,
and binary-only rollback after migration is insufficient.

### Restart canary

The canary has no database, secrets, or business state. Its static response is
deliberately deterministic.

## Data invariants

- Evidence is not mutation authority.
- Model-generated target identity is not authoritative.
- Code rollback does not imply database rollback.
- Secret metadata is not secret plaintext.
- Ambiguous execution is not success and is not safe to retry.
- History, action records, backups, and bug fixtures are durable evidence.

## Future roadmap models

Not implemented: persisted post-action target observations, preview
identity, Git-backed source changes, object storage, MiniVault identities and
rotation, agent capability grants, platform restore manifests, or multi-node
identity/placement.
