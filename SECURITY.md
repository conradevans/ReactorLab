# ReactorLab Security

Status: verified security architecture and invariants as of November 17, 2026.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) remains the master narrative.

## Current verified implementation

### Trust model

```text
Models reason.
Deterministic software authorizes.
Owning services execute narrow operations.
Durable records establish audit history.
```

Model output is untrusted input. Valid JSON or cited evidence does not authorize
infrastructure mutation.

### Network and identity boundaries

- Production origins bind to loopback addresses.
- Public traffic uses Cloudflare Tunnel.
- Cloudflare Access protects administrator surfaces.
- Origins validate JWT signature, issuer, audience, validity, and configured
  administrator identity.
- Privileged MiniAI routes fail closed without valid auth configuration.
- Unsafe administrator POSTs require same-origin behavior.
- PostgreSQL has no published host port.
- Guest endpoints use distinct allowlisted response types and routes.

MiniAI has no `localhost = administrator` bypass. Local position alone is not
administrator identity.

### AI capability boundary

MiniAI has no model-facing arbitrary shell, generic command executor, generic
privileged HTTP mutation, or model-selected target IDs. Its only real executor
is typed application restart through MiniDeploy.

Assistant suggestions require confirmation. Confirmation does not execute.
Reload does not mutate. Ambiguous dispatch is not retried.

### Evidence and action eligibility

ReactorLab's MiniAI contract is GET-only and bounded. It preserves identity,
timestamps, availability, and coverage while excluding sensitive values.
Software owns confidence and restart eligibility.

Restart suggestion requires trusted current evidence for exactly one resolved
application in the closed `exited` condition. Missing, malformed, conflicting,
low-confidence, or incomplete critical evidence fails closed.

### Secret handling

- Database passwords are owner-restricted files, not normal SQLite metadata.
- MiniBase provides binding material only through its private authenticated route.
- Credential responses use no-store semantics.
- Runtime values are absent from Guest data and deployment history.
- Webhook URLs and Access credentials are not committed.
- Public errors are sanitized.

No ReactorLab document should contain a real secret, token, password, private
connection string, or webhook URL.

### Audit

Action proposals/events are reconstructed from trusted storage, not browser
state. Execution support is snapshotted when a proposal is created, preventing
future catalog changes from enabling old actions retroactively. Historical
unsafe proposals and failed investigations remain audit evidence.

### Current limitations

- The Dell is a single-node trust domain.
- MiniVault does not exist; current services protect their own secret files.
- The Phase 4D1 candidate durably labels ambiguous action outcomes, with local
  verification complete and production acceptance pending.
- MiniDeploy restart has no durable operation ID for causal proof.
- Same-Dell backups do not protect against physical storage loss.

## Prohibited architecture

Without explicit architecture review, do not add arbitrary shell execution,
generic model-facing mutation, plaintext secret dumping, model-selected target
identity, retry after ambiguity, confirm-and-execute chaining, reload mutation,
or authentication bypasses.

## Future roadmap security

Planned, not current: MiniVault; least-privilege specialized agents; untrusted
web research; preview isolation before source editing; Git-backed changes;
off-node encrypted recovery; and secure multi-node identity/enrollment.
