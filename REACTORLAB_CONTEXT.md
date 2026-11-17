# ReactorLab Engineering Context

Last major handoff update: November 17, 2026

This document is the canonical engineering handoff for the entire ReactorLab platform.

It exists so that a new Codex session, coding agent, or engineer can understand:

- what ReactorLab is,
- what each repository and service does,
- what has already been implemented,
- what has been proven live,
- why major architectural decisions were made,
- which historical failures materially changed the architecture,
- what safety invariants must never regress,
- what production state must be preserved,
- what remains on the roadmap,
- how autonomous development should be bounded,
- and what the finished ReactorLab product is intended to become.

This is intentionally more detailed than a normal README.

It explains not only WHAT exists, but WHY it exists in its current form.

The repositories and live system remain the ultimate source of truth.

If implementation or Git history is newer than this document:

1. inspect the newer implementation,
2. determine whether the change was intentional,
3. treat verified current code as authoritative,
4. update this context rather than forcing current code to conform to stale documentation.

Never invent implementation facts merely because they seem consistent with the roadmap.

---

# 1. Project Vision

ReactorLab is intended to become:

> A secure, open-source, AI-operated personal developer cloud that turns ordinary commodity computers into an application, database, storage, secrets, observability, development, recovery, automation, and eventually multi-node infrastructure platform.

The project began by turning an unused Dell laptop into a useful personal developer server.

The long-term goal is substantially larger.

A finished ReactorLab installation should allow a developer to take one or more ordinary computers and use them as a coherent private developer cloud without manually managing every:

- Docker container,
- PostgreSQL database/user,
- reverse proxy,
- systemd service,
- deployment lifecycle,
- secret file,
- backup script,
- health monitor,
- recovery workflow,
- incident investigation,
- preview environment,
- source-code change,
- or physical node.

The desired progression is:

```text
old computer
    ↓
private developer server
    ↓
personal developer cloud
    ↓
AI-operated personal cloud
    ↓
secure multi-node personal cloud
```

ReactorLab is not intended to become Kubernetes.

It may borrow useful distributed-systems ideas, but the product should remain:

- understandable,
- opinionated,
- inspectable,
- small enough for an individual developer,
- and much simpler than enterprise cluster infrastructure.

Complexity should be earned by actual requirements.

---

# 2. Defining Engineering Philosophy

The defining idea is not:

> Put an LLM in a server dashboard.

The important idea is:

> Build infrastructure that an AI can safely understand and operate because the infrastructure itself exposes typed evidence, typed capabilities, deterministic authorization, narrow executors, durable state machines, least-privilege identities, and audit trails.

Core principles:

```text
Models reason.

Software decides what is legal.

Models do not choose authoritative infrastructure identity.

Evidence is authoritative.
Model confidence is not.

Agents receive least privilege.

Users authorize dangerous actions.

Executors are narrow and typed.

Secrets are structurally protected.

Ambiguous mutations are never blindly retried.

Current state and causal execution outcome are different facts.

Every important mutation is auditable.

Prompt instructions are not security boundaries.

Completion of one roadmap phase does not authorize the next.
```

If a safety property can be enforced structurally in:

- Go,
- storage,
- policy,
- API design,
- capability design,
- authentication,
- network topology,
- or state-machine validation,

prefer that over relying on the model to follow instructions.

---

# 3. Primary Machines

## The Mac

The user's primary computer and increasingly the primary development/Codex machine.

Refer to it as:

```text
the Mac
```

Current preferred workspace:

```text
~/Coding/ReactorLabWorkspace/
│
├── ReactorLab/
├── MiniDeploy/
├── MiniBase/
├── MiniAI/
└── miniai-restart-canary/
```

Future repositories such as MiniVault should be added to this same workspace.

The Codex project should treat all ReactorLab repositories as related source folders rather than isolated projects.

---

## The Dell

The current ReactorLab production server.

Refer to it as:

```text
the Dell
```

The Dell runs Ubuntu/Linux and currently hosts the live ReactorLab platform.

Important known production locations include:

```text
/srv/miniai
/srv/minibase
```

Other service paths must be verified rather than assumed.

The Dell is production infrastructure.

Do not treat it as an ephemeral CI machine.

Do not casually:

- reset databases,
- delete History,
- delete action audit records,
- remove old bug fixtures,
- overwrite service configuration,
- or reboot the machine merely because deployment verification is inconvenient.

---

# 4. Current Workspace Repositories

## ReactorLab

Repository:

```text
conradevans/ReactorLab
```

Purpose:

Umbrella platform/control-plane repository.

ReactorLab provides or should increasingly provide:

- unified platform state,
- platform-level observability,
- recovery integration,
- cross-service read contracts,
- shared architecture,
- roadmap documentation,
- eventual node-level concepts,
- and the cohesive product experience over the underlying Mini services.

This is the appropriate home for canonical platform documentation such as:

```text
REACTORLAB_CONTEXT.md
ROADMAP.md
ARCHITECTURE.md
SECURITY.md
DECISIONS.md
PRODUCTION.md
```

Known baseline at this handoff:

```text
b456264543c06ee84b088fd51433ca16461a2b28
feat: add MiniAI universal read contract
```

Important older platform commit:

```text
eac1f8288ffeb2d4ae89e667a831cae97b958279
feat: add automatic shutdown recovery
```

Always verify current HEAD.

---

## MiniDeploy

Repository:

```text
conradevans/MiniDeploy
```

Purpose:

Application deployment and runtime platform.

MiniDeploy owns concepts such as:

- applications,
- deployment lifecycle,
- source identity,
- Docker/container runtime,
- health,
- restarts,
- rollback,
- deployment history,
- domains/public visibility,
- service-level runtime state,
- and future preview environments.

Known baseline:

```text
c4e03e98be3d9811503c4dbbc38f417e131c6b65
feat: add exact deployment source identity
```

Other important historical work includes:

```text
cd2e4fd
feat: add deployment guest visibility controls

f9ec859
feat: add multi-view admin dashboard
```

Always verify current HEAD.

---

## MiniBase

Repository:

```text
conradevans/MiniBase
```

Purpose:

Managed application data platform.

MiniBase currently focuses on PostgreSQL lifecycle and is planned to later include object/file storage.

Known baseline:

```text
05ced46bd60b31ba87cac852c985717465694fc2
chore: reorder MiniBase admin navigation
```

Important historical work includes:

```text
9503649
feat: add database explorer

53b14e5
feat: add database guest visibility controls
```

Other important previously completed work includes:

- managed PostgreSQL provisioning,
- project database ownership,
- backup,
- restore,
- lifecycle safety,
- automatic application bindings,
- public/private origin separation,
- and observability integration.

Always verify current HEAD.

---

## MiniAI

Repository:

```text
conradevans/MiniAI
```

Purpose:

Local evidence-grounded infrastructure assistant and future agent orchestrator.

Known baseline:

```text
b55e4a2e7ac0bd53e89da1cb50b6bf80f164799b
fix: harden assistant restart eligibility
```

Immediately preceding important commits:

```text
cdbc86bd9e467cb4f267ca90403132157bcfe210
feat: add MiniAI action controls

766270ab2a79d1fe880491eb96abd8451d74e6cc
fix: keep Phase 2C streams alive during reasoning
```

MiniAI is the current active development focus.

---

## miniai-restart-canary

Repository:

```text
conradevans/miniai-restart-canary
```

Purpose:

Disposable live regression application specifically for safe testing of real MiniAI restart actions.

Known commit:

```text
d0ac280eb2af3e5f8d9069c0143b43bc2df23751
chore: add MiniAI restart canary
```

This should remain deliberately simple and deterministic.

Do not turn the canary into a complex application.

Its job is to absorb mutation testing so real applications do not need to.

---

# 5. Long-Term Product Components

The intended major platform components are:

```text
ReactorLab
│
├── MiniDeploy
├── MiniBase
├── MiniVault
└── MiniAI
```

## MiniDeploy

Application deployment and compute lifecycle.

## MiniBase

Databases, backups, and eventually object/file storage.

## MiniVault

Future secrets, credentials, API keys, service identity, bindings, and security layer.

Earlier temporary naming such as `ReactorVault` is superseded.

The intended name is:

```text
MiniVault
```

## MiniAI

Intelligence, evidence reasoning, safe infrastructure actions, and eventual specialized-agent orchestration.

---

# 6. Why the Components Are Separate

The service boundaries are intentional.

## MiniDeploy owns application lifecycle

MiniDeploy should answer:

```text
What is deployed?
Where did it come from?
What exact commit is running?
Is it healthy?
How was it deployed?
Can it restart?
Can it roll back?
```

MiniAI should request application-level actions through MiniDeploy rather than manipulate containers directly.

---

## MiniBase owns managed application data

MiniBase should answer:

```text
Which database belongs to this project?
Which application is attached?
When was it backed up?
Can it be restored?
What is its current state?
What storage belongs to this application?
```

Database contents must not silently follow code rollback.

Code history and data history are separate timelines.

---

## MiniVault will own secrets and identity

MiniVault should answer:

```text
Does this application have the required secret?
Is the binding valid?
Has the credential expired?
When was it last rotated?
Which service identity owns this token?
```

MiniAI should generally not be able to answer:

```text
What is the plaintext secret?
```

---

## MiniAI owns intelligence

MiniAI should consume typed APIs from the other components.

It should not become the authoritative state store for:

- deployments,
- databases,
- secrets,
- node configuration,
- or application lifecycle.

This separation means:

- infrastructure remains usable without AI,
- MiniAI failures do not corrupt platform truth,
- model mistakes do not become state,
- future agents can use narrowly scoped capabilities,
- and AI operation can remain subordinate to deterministic platform rules.

---

# 7. Final Conceptual Architecture

```text
                         REACTORLAB
                 AI-Operated Personal Cloud
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
   MiniDeploy          MiniBase          MiniVault
        │                  │                  │
 Applications         PostgreSQL          Secrets
 Containers           Object Storage      API Keys
 Previews             Backups             Identities
 Domains              Bindings            Rotation
        │                  │                  │
        └──────────────────┼──────────────────┘
                           │
                         MiniAI
                           │
               Agent Orchestration Layer
                           │
     ┌──────────┬──────────┼──────────┬──────────┐
     │          │          │          │          │
 Investigation Coding  Deployment  Security   Recovery
    Agent       Agent     Agent      Agent      Agent
     │          │          │          │          │
     └──────────┴──────────┼──────────┴──────────┘
                           │
             Typed Capability / Policy Layer
                           │
                 Authorization + Audit
                           │
              ┌────────────┼────────────┐
              │            │            │
            Node 1       Node 2       Node N
```

The multi-node layer is intentionally late.

Single-node ReactorLab should become excellent first.

---

# 8. MiniDeploy Foundation

MiniDeploy began as the application/deployment platform.

It has evolved beyond simply starting containers.

Important completed concepts include:

- application definitions,
- Docker/container runtime,
- full-stack application lifecycle,
- multiple services per project where needed,
- deployment state,
- restart,
- rollback,
- deployment history,
- public/Guest visibility,
- admin dashboard,
- exact deployment source identity.

---

# 9. Exact Deployment Source Identity

One major architectural improvement was preserving exact source provenance.

A deployment should not merely say:

```text
branch = main
```

because `main` moves.

The durable truth should include:

```text
repository
requested ref
branch/ref
exact commit SHA
activation time
deployment identity
```

This becomes crucial for future questions such as:

```text
What exact version is running?

Did this problem begin after deployment X?

Is production behind the checkout?

Can a preview recreate exactly what production is running?
```

This also underpins future controlled source editing.

---

# 10. Why MiniDeploy Owns Restart

The first real MiniAI action is:

```text
restart_application
```

MiniAI does not restart Docker containers directly.

Instead:

```text
MiniAI
    ↓
typed restart intent
    ↓
MiniDeploy
    ↓
application lifecycle
```

Known MiniDeploy route used by MiniAI:

```text
POST /deployments/{app}/restart
```

The reason is ownership.

MiniDeploy understands:

- the application,
- its container(s),
- associated lifecycle rules,
- database attachment state,
- deployment metadata,
- and what "restart application" actually means.

MiniAI should not bypass that by issuing:

```text
docker restart
systemctl restart
shell commands
```

---

# 11. MiniBase Foundation

MiniBase is a managed database control plane.

The architecture intentionally favors:

- one private persistent PostgreSQL service,
- per-project databases,
- per-project users,
- managed credentials,
- application attachment,
- managed DATABASE_URL injection,
- backup/restore lifecycle,
- persistent metadata.

PostgreSQL should not expose port 5432 publicly by default.

Database access should remain private to the platform unless a future reviewed use case explicitly requires exposure.

---

# 12. MiniBase Lifecycle Safety

MiniBase database lifecycle operations have been live-tested.

Important proven behavior included:

- attached database deletion blocked,
- backup,
- restore,
- restore into new database,
- replace/restore behavior,
- detach,
- delete,
- Activity preservation,
- unrelated database/application state unaffected.

Deletion was intentionally hardened.

Important design properties include:

- serialize destructive lifecycle work,
- prevent attachment races,
- keep metadata until destructive work succeeds,
- delete metadata last,
- retain retryability,
- sanitize public errors.

The principle is:

> Data deletion is a lifecycle operation, not merely DELETE SQL.

---

# 13. MiniBase Backup Philosophy

MiniBase supports scheduled persistent backups.

Historical configuration included daily backup around:

```text
03:00 UTC
```

Treat schedule values as configuration, not permanent constants.

Important rule:

```text
application rollback
≠
database rollback
```

Rolling application code back must never silently roll database contents backward.

---

# 14. MiniBase Public/Private Boundary

Historical MiniBase listeners include:

```text
private management:
127.0.0.1:9100

public origin:
127.0.0.1:9103
```

Cloudflare Tunnel routes the public hostname to the public origin.

Public routes and admin routes are intentionally different trust surfaces.

Public origin does not mean every endpoint is public.

---

# 15. MiniBase Secure Origin

MiniBase secure public origin work includes a historical commit around:

```text
f15fbc7
feat: add secure public ReactorLab origin
```

Important principles:

- separate loopback listeners,
- Cloudflare Tunnel,
- Access protection for admin surfaces,
- origin JWT validation,
- Guest/public routes intentionally exposed,
- management routes protected.

Do not collapse this boundary for convenience.

---

# 16. MiniBase Observability

MiniBase provides observability information to ReactorLab.

The platform should expose enough metadata to reason about databases without exposing:

- database passwords,
- connection secrets,
- private URLs,
- raw credentials.

This principle later directly informed MiniAI evidence design.

---

# 17. ReactorLab Unified Control Plane

ReactorLab should increasingly feel like one product despite multiple services.

The accepted UI direction is:

```text
ReactorLab overview
    ↓
projects/applications
    ↓
deployment + database + health + AI + recovery
```

Detailed service-specific dashboards may remain.

The goal is not to erase service ownership.

The goal is to provide a coherent product above the boundaries.

---

# 18. Historical ReactorLab Scale

At one earlier production milestone, ReactorLab managed approximately:

```text
6 deployments
8 containers
2 PostgreSQL databases
```

These values are historical proof of real use.

Do not hardcode them.

The significance is that ReactorLab has already operated multiple real workloads rather than only toy examples.

---

# 19. ReactorLab Recovery System

ReactorLab includes automatic machine-level recovery/incident tracking.

Important historical commit:

```text
eac1f8288ffeb2d4ae89e667a831cae97b958279
feat: add automatic shutdown recovery
```

This work was motivated by unexpected Dell shutdown/reboot behavior.

The recovery system includes concepts such as:

- outage sessions,
- durable recovery events,
- watchdog/RTC recovery behavior,
- Activity integration,
- overview integration,
- notifications,
- retries/outbox behavior,
- historical recovery tracking.

The system should distinguish:

```text
application failure
service failure
machine reboot/outage
```

These are different classes of failure.

---

# 20. Reboot Recovery

Live reboot testing previously verified recovery of important platform services and state.

The broader design requirement is:

> ReactorLab must survive host reboot as a platform.

Future installer and multi-node work should preserve this property.

---

# 21. ReactorLab Universal Read Contract

ReactorLab later gained the structured read contract that supports MiniAI.

Current known commit:

```text
b456264
feat: add MiniAI universal read contract
```

This marks an important architecture shift.

MiniAI should not understand infrastructure by running arbitrary shell commands and scraping terminal output.

ReactorLab and its services should expose typed state.

---

# 22. Why Typed Evidence Exists

Raw shell output requires a model to infer:

- identity,
- freshness,
- authority,
- semantics,
- completeness,
- absence,
- relationships.

Typed evidence can explicitly encode:

```text
subject kind
subject ID
observation type
value
availability
timestamp
coverage
error category
```

This makes the system:

- more testable,
- safer,
- more deterministic,
- easier to audit,
- less prompt-sensitive,
- and suitable for action eligibility.

Typed evidence is foundational.

---

# 23. MiniAI Original Role

MiniAI is a local-first ReactorLab assistant running on the Dell.

Primary local model:

```text
qwen3:8b-q4_K_M
```

Fallback:

```text
qwen3:4b
```

Intended hierarchy:

```text
8B = primary high-quality reasoning

4B = safe fallback / degraded mode
```

Infrastructure questions should not require sending private platform state to an external model.

---

# 24. MiniAI CPU Power Lease

The Dell is not dedicated inference hardware.

MiniAI includes a protected model power lease.

During heavier model work it may temporarily request a higher CPU power profile and restore the normal profile afterward.

Historical logs include:

```text
cpu_power_lease=acquired
cpu_power_profile=3ghz_power

cpu_power_lease=restored
cpu_power_profile=normal_restored
```

This exists for hardware reasons.

Do not remove it merely because it looks unrelated to AI logic.

---

# 25. MiniAI Phase 0 — Architecture Foundation

Phase 0 refactored MiniAI toward typed capabilities and evidence-driven reasoning.

Important historical architecture commit:

```text
e0e13e13492486c7ac0990ce65751f809e213f1b
```

The original simple chat architecture would not safely scale to infrastructure operation.

Phase 0 laid the foundation for:

- typed capabilities,
- deterministic routing,
- evidence packets,
- model policy,
- History,
- and future actions.

---

# 26. MiniAI Phase 1 — Read-Only Typed Evidence

MiniAI gained bounded read capabilities covering areas such as:

- application context,
- deployment state,
- runtime logs,
- deployment logs,
- deployment history,
- repository files/search,
- host metrics/history,
- temperatures,
- infrastructure events,
- databases,
- backups,
- recovery,
- platform overview.

These capabilities should remain:

```text
read-only
typed
bounded
```

Do not replace them with unrestricted shell access.

---

# 27. MiniAI Phase 2 — Investigation Pipeline

The intended investigation architecture is:

```text
question
   ↓
deterministic router
   ↓
bounded evidence plan
   ↓
typed reads
   ↓
evidence reduction
   ↓
EvidencePacket
   ↓
Qwen reasoner
   ↓
strict validation
   ↓
answer
```

MiniAI should not simply dump all server state into Qwen.

---

# 28. Deterministic Routing

Routes exist for bounded classes of investigation such as:

- current platform health,
- thermal state,
- restart investigation,
- current application state,
- application performance,
- deployment correlation,
- database investigation,
- repository investigation.

Routing is software-owned.

The model does not select authoritative application identity.

---

# 29. Temporal Reasoning

Software resolves questions involving time such as:

```text
last 24 hours
today
yesterday
specific incident window
```

The reasoner receives the resolved scope.

It must not invent a different time window.

This matters because:

```text
"No failures"
```

is meaningless without:

```text
during what period
using what evidence
with what coverage
```

---

# 30. Application Runtime Trend Fix

A routing gap was discovered for application + runtime trend questions.

Example:

```text
Has MyScheduler had any failures in the last 24 hours?
```

The fix added an application/runtime trend investigation route.

Typical evidence includes:

- app context,
- application history,
- runtime logs,
- service history,
- host history.

The system should gather only relevant evidence.

Do not make every investigation query every service.

---

# 31. Evidence Coverage

MiniAI distinguishes evidence coverage from certainty.

If logs are incomplete, the correct answer style is:

```text
No failures were recorded in the available evidence.

Runtime-log coverage was incomplete.

Confidence: Medium.
```

Not:

```text
The application definitely had no failures.
```

Absence from incomplete evidence is not proof of non-occurrence.

---

# 32. Confidence Ownership

Confidence is software-owned.

Known levels:

```text
High
Medium
Low
```

The model must not set its own confidence.

Software derives it from evidence properties.

The model renders the evidence-supported conclusion.

---

# 33. One-Pass Reasoner

Phase 2C is designed around one bounded final reasoner call.

The model receives:

- question,
- answer instructions,
- bounded EvidencePacket,
- output schema.

The reasoner does not get arbitrary tool loops.

---

# 34. Reasoner Validation

Model output is untrusted.

Validation covers concepts such as:

- required keys,
- unknown fields,
- duplicate fields,
- length limits,
- evidence IDs,
- malformed JSON,
- trailing JSON,
- incomplete responses,
- output-limit completion,
- attempted tool calls,
- forbidden confidence wording.

Valid JSON alone is insufficient.

---

# 35. Optional Action Suggestion Parsing

A historical bug showed that duplicate optional `action_suggestion` fields could invalidate an otherwise valid answer.

Current intended behavior:

```text
duplicate required answer field
→ invalid

duplicate optional action_suggestion
→ discard action suggestion
→ preserve valid answer
```

Optional action metadata must never destroy a valid evidence answer.

---

# 36. SSE Keepalive

Qwen 8B can take over a minute to reason.

A browser network failure exposed missing keepalive propagation in Phase 2C.

Fix:

```text
766270ab2a79d1fe880491eb96abd8451d74e6cc
fix: keep Phase 2C streams alive during reasoning
```

SSE keepalives are emitted approximately every 15 seconds during long reasoning.

Do not remove them.

---

# 37. MiniAI Phase 3 — Durable History

Investigations are persisted in SQLite.

History records operational facts such as:

- request,
- result,
- timestamps,
- route,
- model,
- confidence,
- reads,
- reasoner calls,
- evidence rounds,
- validation,
- tokens,
- timing,
- errors.

This allows later answers to questions like:

```text
What did MiniAI investigate?
Which model ran?
Which evidence was gathered?
Did inference fail?
Did an action come from this investigation?
```

History is durable audit evidence.

---

# 38. Production MiniAI Database

Known path:

```text
/var/lib/miniai/miniai.db
```

Known schema version at this handoff:

```text
3
```

Verify before schema changes.

Avoid migrations unless necessary.

Never wipe production state merely to simplify development.

---

# 39. Why Phase 4 Was Deliberately Slow

Phase 4 introduced real infrastructure mutation.

Before Phase 4, a bad MiniAI answer could mislead the user.

After Phase 4, a bad decision could change infrastructure.

Therefore mutation was introduced incrementally:

```text
typed policy
→ persistence
→ authentication
→ API
→ executor
→ execution
→ model suggestion
→ UI
→ semantic hardening
```

This was intentional.

Do not replace the architecture with a broad "tool calling" shortcut.

---

# 40. Phase 4A1 — Typed Action Policy

Important commit:

```text
6a73213
feat: add typed MiniAI action policy foundation
```

Origins:

```text
direct_user
assistant_suggested
```

Safety classes:

```text
reversible
recoverable
compensatable
irreversible
```

Current conceptual policy:

```text
DIRECT USER

reversible/recoverable
→ authorized

compensatable
→ confirmation

irreversible
→ confirmation + warning


ASSISTANT-SUGGESTED

all
→ confirmation

irreversible
→ confirmation + warning
```

---

# 41. Action Lifecycle

Core state machine:

```text
proposed
   ↓
awaiting_confirmation OR authorized

awaiting_confirmation
   ↓
authorized OR cancelled

authorized
   ↓
executing

executing
   ↓
succeeded OR failed
```

An ambiguous execution may intentionally remain:

```text
executing
```

with an outcome equivalent to:

```text
outcome_unknown
```

rather than inventing certainty.

---

# 42. Why Confirm and Execute Are Separate

Assistant-suggested actions deliberately require two separate user interactions.

```text
Confirm
```

means:

```text
I authorize this action.
```

It does not mean:

```text
Execute immediately inside the same request.
```

Then:

```text
Execute
```

is a separate mutation.

This provides:

- clearer intent,
- auditable transitions,
- reduced accidental execution,
- better UI communication,
- stronger testability.

Confirm must never call Execute.

---

# 43. Phase 4A2 — Durable Actions

Commit:

```text
5090370
feat: persist MiniAI action proposals and audit history
```

SQLite includes durable:

```text
action_proposals
action_events
```

Action state is reconstructed from trusted storage.

Browser state is not authoritative.

---

# 44. Phase 4A2.5 — Admin Authentication

Commit:

```text
d39a7ee
feat: add MiniAI admin authentication boundary
```

Privileged routes use Cloudflare Access JWT validation at the MiniAI origin.

Security includes:

- Access assertion validation,
- issuer validation,
- audience validation,
- exact configured admin identity,
- RS256/JWKS validation,
- same-origin requirement for unsafe POSTs,
- fail-closed privileged routes if configuration is missing.

No loopback admin bypass.

---

# 45. Why No Localhost Bypass Exists

A rule such as:

```text
localhost = admin
```

would mean a compromised local process could call privileged APIs.

Administrator identity should not be inferred merely from local network location.

This is defense in depth.

---

# 46. Action API

Current authenticated action API includes:

```text
POST /api/admin/actions

GET /api/admin/actions?history_id=<history-id>

GET /api/admin/actions/{action_id}

GET /api/admin/actions/{action_id}/events

POST /api/admin/actions/{action_id}/confirm

POST /api/admin/actions/{action_id}/cancel

POST /api/admin/actions/{action_id}/execute
```

The History lookup is intentionally read-only and strict.

---

# 47. Existing Action Catalog

Known kinds:

```text
restart_application
rollback_deployment
deploy_repository
```

Current safety classification:

```text
restart_application
→ recoverable

rollback_deployment
→ irreversible

deploy_repository
→ compensatable
```

IMPORTANT:

Only:

```text
restart_application
```

currently has a real executable backend.

Catalog presence must not automatically imply execution support.

---

# 48. Phase 4B1 — Narrow Restart Executor

Commit:

```text
8f7fef7
feat: add MiniAI restart executor foundation
```

The executor is deliberately application-specific.

MiniAI uses a typed MiniDeploy restarter.

Known MiniDeploy local origin:

```text
http://127.0.0.1:9000
```

Known request:

```text
POST /deployments/{app}/restart
```

---

# 49. Forbidden Generic Mutation Architecture

Do not introduce:

```text
executeShell(command)
genericExecutor
dockerCommand(...)
systemctlCommand(...)
arbitraryHTTPMutation(...)
```

as model-facing infrastructure actions.

A generic executor would collapse every security boundary into one unrestricted primitive.

Each mutation should get its own narrowly typed executor.

---

# 50. Restart Failure Categories

Definitive pre-mutation failures may safely transition to `failed`.

Historical examples include:

```text
404 deployment not found
409 database detached
```

Other failures may be ambiguous if mutation may already have begun.

Examples:

```text
timeout after dispatch
5xx after dispatch
connection reset after request accepted
response read failure
```

These must not automatically retry.

---

# 51. Why Ambiguous Mutations Must Not Retry

Example:

```text
MiniAI sends restart
MiniDeploy performs restart
network fails before reply
MiniAI sees transport error
```

If MiniAI retries:

```text
restart #1
restart #2
```

Transport uncertainty has caused duplicate mutation.

Therefore:

```text
unknown outcome
≠
safe retry
```

This rule should apply to future:

- deploy,
- rollback,
- restore,
- delete,
- rotation,
- self-healing,
- multi-node operations.

---

# 52. Phase 4B2 — Controlled Execution

Commit:

```text
8e4cda8
feat: add controlled MiniAI restart execution
```

Execution requires both:

```text
current action catalog supports execution
```

AND:

```text
persisted proposal supports execution
```

This prevents future catalog changes from retroactively making old actions executable.

---

# 53. First Real Direct Restart

Known action:

```text
action_ec81dad4db3e2a1d536c326bb78860a6
```

Target:

```text
MyScheduler
```

Observed event path:

```text
policy_authorized_on_creation
null → authorized

execution_started
authorized → executing

execution_succeeded
executing → succeeded
```

Container IDs stayed the same while uptime reset.

That proved a real restart rather than replacement deployment.

Duplicate execution returned conflict rather than restarting twice.

---

# 54. Phase 4C1 — Assistant-Suggested Restart

Commit:

```text
75e70d6
feat: add MiniAI assistant restart suggestions
```

Qwen may optionally emit a structured intent conceptually equivalent to:

```text
restart_current_application
```

The model can supply:

- intent,
- bounded reason,
- evidence references.

The model does NOT supply the authoritative application target.

---

# 55. Why Go Binds the Target

If evidence concerns:

```text
myscheduler
```

and the model outputs:

```text
restart golf-mullet
```

the model must not be trusted.

The safe architecture is:

```text
model intent
+
trusted deterministic subject resolution
=
action proposal
```

The model suggests WHAT kind of thing may be useful.

Software determines WHICH actual platform object is eligible.

---

# 56. Assistant-Suggested Policy

Assistant restart actions persist as:

```text
origin = assistant_suggested
safety = recoverable
state = awaiting_confirmation
confirmation_required = true
warning_required = false
supports_execution = true
```

Recommendation is not authorization.

Authorization is not execution.

---

# 57. Restart Canary

Disposable test application:

```text
miniai-restart-canary
```

Container:

```text
minideploy-miniai-restart-canary
```

Known public hostname:

```text
https://miniai-restart-canary.reactorlab.dev
```

Historical container ID during Phase 4C testing:

```text
be428f2dc3b8
```

Never assume the container ID remains current.

Verify it.

---

# 58. Original Assistant Restart Proof

Known action:

```text
action_a8047de7d06225c0e6db08be367fe651
```

History:

```text
4cf81abcc9a363c44011597ce1602142
```

Canary was stopped.

MiniAI observed `exited`.

MiniAI suggested restart.

User confirmed.

Canary remained exited.

User separately executed.

Canary became healthy.

This proved basic recommendation → confirmation → execution separation.

---

# 59. Phase 4C2 — UI Controls

Commit:

```text
cdbc86bd9e467cb4f267ca90403132157bcfe210
feat: add MiniAI action controls
```

UI state expectations:

```text
awaiting_confirmation
→ Confirm restart
→ Dismiss

authorized
→ Execute restart

executing
→ no duplicate mutation controls

succeeded
→ terminal

failed
→ terminal/recovery presentation

cancelled
→ terminal
```

---

# 60. Frontend Request Mapping

```text
confirm:
POST /api/admin/actions/{id}/confirm
body {"confirm":true}

dismiss:
POST /api/admin/actions/{id}/cancel

execute:
POST /api/admin/actions/{id}/execute
```

No confirm-to-execute chaining.

---

# 61. Mutation Concurrency

Only one mutation should be in flight per action card.

This protects against:

- double clicks,
- races,
- duplicate transition requests.

---

# 62. Safe Reconciliation

For:

```text
409
network ambiguity
no HTTP status
5xx
unexpected response
```

the frontend should not automatically resend the mutation.

It should reconcile via:

```text
GET /api/admin/actions/{id}
```

This pattern should be reused for future action types.

---

# 63. Reload Behavior

Page/chat reload must be:

```text
read state
render state
```

Never:

```text
confirm
cancel
execute
retry
```

Reload must remain read-only.

---

# 64. Important Semantic Safety Bug

During Phase 4C2 testing, MiniAI answered that the canary was healthy/running but still produced a structurally valid restart recommendation.

Known unsafe historical action:

```text
action_b845e58f1426fb3f6362451bc6c71ae8
```

It remained awaiting confirmation.

The user dismissed it.

No restart occurred.

The record is deliberately retained.

Do not delete it.

---

# 65. Why the Healthy-App Bug Mattered

The original eligibility logic checked structure and references but did not independently prove that current evidence justified restart.

This demonstrated:

```text
valid model structure
≠
valid infrastructure action
```

Human confirmation saved the system from an unnecessary restart, but the backend still needed semantic enforcement.

---

# 66. Phase 4C1.1 — Restart Semantic Hardening

Current production baseline:

```text
b55e4a2e7ac0bd53e89da1cb50b6bf80f164799b
fix: harden assistant restart eligibility
```

Restart eligibility is now deterministic and evidence-owned.

Current principles include:

- trusted supported route,
- exactly one trusted application,
- current evidence required,
- wrong-target evidence fails closed,
- malformed evidence fails closed,
- missing evidence fails closed,
- unknown state fails closed,
- conflicting current states fail closed,
- historical failures alone insufficient,
- runtime failures alone insufficient,
- Low confidence suppresses suggestion,
- incomplete critical evidence suppresses suggestion.

Current closed trigger:

```text
status == "exited"
```

---

# 67. Why Current State Is Required

Historical failure does not imply current restart need.

Example:

```text
10:00 app crashes
10:01 system recovers it
11:00 app is running
```

A 10:00 crash does not justify an 11:00 restart.

Current evidence must independently show a restart-worthy condition.

---

# 68. Healthy-Path Live Regression

After semantic hardening:

```text
Is miniai-restart-canary healthy right now, and what should be done next?
```

while canary was running produced:

- healthy/running answer,
- no action card,
- no proposal,
- no event.

This proves the unsafe healthy-app recommendation bug was closed.

---

# 69. Final Phase 4C2 End-to-End Proof

The canary was deliberately stopped.

MiniAI correctly observed:

```text
exited
```

and suggested restart.

Action:

```text
action_0ef0b9ae027bc192714520136c5a61ca
```

History:

```text
1d1dc9d03d8d0f6008bb7a18c37a7c41
```

---

# 70. Confirmation-Only Proof

After user clicked Confirm:

```text
state = authorized
```

Events:

```text
proposal_created
null → awaiting_confirmation

confirmation_granted
awaiting_confirmation → authorized
```

At this exact point the canary remained:

```text
Exited
```

Therefore:

```text
CONFIRM ≠ EXECUTE
```

was live-proven.

---

# 71. Separate Execution Proof

After a separate Execute click:

```text
state = succeeded
```

Events:

```text
proposal_created
null → awaiting_confirmation

confirmation_granted
awaiting_confirmation → authorized

execution_started
authorized → executing

execution_succeeded
executing → succeeded
```

The canary became:

```text
Up
```

HTTP returned:

```text
MiniAI Restart Canary
Healthy.
```

Container ID remained unchanged.

Therefore this was a real restart, not replacement deployment.

---

# 72. Reload Proof

Before reload:

```text
state = succeeded
proposals = 5
events = 15
```

After reload:

```text
state = succeeded
proposals = 5
events = 15
```

No new mutation occurred.

No new event appeared.

Canary remained healthy.

Phase 4C2 is complete and live-proven.

---

# 73. Production Action Fixtures

Known important production action records include:

```text
action_6b29ccf5ff1eb4312a2c5b6769be8e90
rollback smoke
cancelled

action_ec81dad4db3e2a1d536c326bb78860a6
direct-user MyScheduler restart
succeeded

action_a8047de7d06225c0e6db08be367fe651
legitimate assistant canary restart
succeeded

action_b845e58f1426fb3f6362451bc6c71ae8
historical unsafe healthy-app recommendation
cancelled

action_0ef0b9ae027bc192714520136c5a61ca
final Phase 4C2 assistant restart proof
succeeded
```

These records are historical evidence.

Do not clean them up merely because some represent bugs.

---

# 74. Cold Qwen Failure

During one canary investigation, a request failed with a network error.

Known History:

```text
9410d4f815c8b8fa3f68e0172517fcb2
```

Recorded properties included:

```text
status = canceled
answer_validation = transport_error
error_category = client_canceled
model = qwen3:8b-q4_K_M
reasoner_calls = 1
generated_tokens = 0
```

Ollama:

- cold-loaded Qwen 8B,
- processed most of the prompt,
- returned HTTP 500 after approximately 49.95 seconds.

No OOM was observed.

No action was persisted.

Canary remained exited.

A later attempt succeeded.

---

# 75. Lesson from the Cold Failure

Inference failure must not create infrastructure authorization or mutation.

The action subsystem failed safely.

Do not confuse:

```text
inference retry
```

with:

```text
mutation retry
```

These have completely different risk profiles.

---

# 76. Current MiniAI Client Behavior

The general HTTP client has historically used a long timeout, roughly:

```text
10 minutes
```

The reasoner emits SSE keepalives.

Therefore the ~50 second cold failure was not treated as a known intentional MiniAI timeout.

If it becomes reproducible, diagnose before changing timeout values.

---

# 77. Completed MiniAI Roadmap

```text
Phase 0
Architecture foundation
COMPLETE

Phase 1
Typed read-only evidence
COMPLETE

Phase 2
Bounded investigation/reasoning
COMPLETE

Phase 3
Durable History
COMPLETE

Phase 4A1
Typed action policy
COMPLETE

Phase 4A2
Durable action storage/audit
COMPLETE

Phase 4A2.5
Admin authentication
COMPLETE

Phase 4A3
Authenticated action API
COMPLETE

Phase 4B1
Restart executor foundation
COMPLETE

Phase 4B2
Controlled restart execution
COMPLETE + LIVE PROVEN

Phase 4C1
Assistant restart suggestions
COMPLETE

Phase 4C1.1
Deterministic semantic restart eligibility
COMPLETE + LIVE PROVEN

Phase 4C2
Interactive confirmation/execution UI
COMPLETE + LIVE PROVEN
```

---

# 78. Immediate Next Milestone

Current roadmap ceiling unless explicitly changed:

```text
Phase 4D1
```

Title:

> Action failure, ambiguity, reconciliation, and recovery hardening.

Do not begin later roadmap implementation unless explicitly authorized.

---

# 79. Phase 4D1 Purpose

The happy path is proven:

```text
suggestion
→ awaiting_confirmation
→ authorized
→ executing
→ succeeded
```

Now unhappy paths need the same level of quality.

Primary cases:

```text
authorized
→ executing
→ failed
```

and:

```text
authorized
→ executing
→ outcome_unknown
```

The system must never lie about whether an action occurred.

---

# 80. Phase 4D1 Acceptance Criteria

## Definitive failure

If MiniDeploy definitively rejects execution before mutation:

```text
action → failed
```

The UI should clearly explain failure.

No automatic mutation retry.

---

## Ambiguous execution

If MiniAI cannot determine whether execution completed:

```text
outcome_unknown
```

must remain explicitly uncertain.

Do not convert transport ambiguity into success or failure without evidence.

---

## Reconciliation

Provide a safe GET-only reconciliation/refresh path.

No mutation retry as part of reconciliation.

---

## State vs causation

If an ambiguous restart is followed by:

```text
application = running
```

MiniAI may state:

```text
The application is currently running.
```

It may not state:

```text
The restart definitely succeeded.
```

unless execution evidence proves it.

---

## Reload safety

Reloading failed or ambiguous actions must remain read-only.

---

## Duplicate execution

Multiple Execute clicks/requests must not produce multiple restarts.

---

## Auditability

Action state should expose useful operational data such as:

- execution outcome,
- failure category,
- timestamps,
- reconciliation information where appropriate.

---

## Live proof

Use:

```text
miniai-restart-canary
```

for bounded mutation testing.

---

# 81. Phase 4D2

Planned:

> Action observability and recovery UX.

Potential scope:

- richer action detail panels,
- failure categories,
- execution timestamps,
- execution duration,
- post-action observation,
- reconciliation state,
- safe recovery guidance.

Do not implement 4D2 during a 4D1 task.

---

# 82. Phase 4D3

Planned:

> Second real typed action.

Do not add another executor until restart is reference-quality.

Each new executable action should have:

```text
typed ActionKind
typed target
safety class
deterministic policy
durable proposal
audit events
eligibility
narrow executor
bounded failure semantics
tests
live proof where appropriate
```

---

# 83. Phase 5 — Preview Deployments

Future MiniAI flow:

```text
repository
   ↓
ref
   ↓
exact commit
   ↓
build
   ↓
isolated preview
   ↓
inspect/test
```

Preview identity must preserve:

```text
repository
requested ref
exact SHA
origin application
preview ID
```

Production must not be touched by default.

---

# 84. Why Preview Comes Before Source Editing

MiniAI needs a safe verification environment before it gains code-writing ability.

Desired sequence:

```text
investigate production
   ↓
candidate fix
   ↓
preview
   ↓
test
   ↓
human approval
   ↓
production
```

---

# 85. Phase 6 — Controlled Source Editing

Source mutation should be Git-backed.

Initial preferred model:

```text
branch
→ edit
→ diff
→ tests
→ preview
→ approval
→ merge/deploy
```

Not:

```text
edit production checkout directly
```

Git provides:

- history,
- diff,
- rollback,
- attribution,
- review,
- reproducibility.

---

# 86. Phase 7 — Controlled Web Research

MiniAI should eventually research:

- official docs,
- GitHub issues,
- releases,
- CVEs,
- dependency compatibility,
- framework docs.

But:

```text
web evidence = untrusted evidence
```

A webpage must never authorize an infrastructure action.

---

# 87. Phase 8 — Evaluation and Hardening

Build a repeatable evaluation suite for:

- hallucinations,
- grounding,
- unsupported claims,
- causal claims,
- negative conclusions,
- partial evidence,
- conflicting evidence,
- malformed outputs,
- adversarial evidence,
- action authorization,
- target binding,
- confirmation boundaries,
- transport failures,
- ambiguous execution,
- 8B vs 4B behavior,
- latency.

The objective is predictable system behavior, not merely attractive answers.

---

# 88. Phase 9 — Log Explorer / Observability

Future ReactorLab observability should include:

- runtime logs,
- deployment logs,
- infrastructure events,
- recovery events,
- host metrics,
- temperature metrics,
- search,
- filters,
- time correlation.

MiniAI should eventually link conclusions directly to these records.

---

# 89. Phase 10 — Self-Healing

Automatic recovery should reuse the Phase 4 action architecture.

Do not create a second mutation system.

Conceptual flow:

```text
deterministic health condition
   ↓
recovery policy
   ↓
typed action
   ↓
executor
   ↓
audit
```

Self-healing should include:

- cooldowns,
- restart-loop protection,
- thresholds,
- escalation,
- clear audit history.

---

# 90. Phase 11 — MiniBase Object Storage

MiniBase should eventually add object/file storage.

Potential concepts:

- buckets,
- objects,
- application bindings,
- quotas,
- backups,
- public/private object access.

Do not create unnecessary Mini services when MiniBase can coherently own managed data.

---

# 91. Phase 12 — MiniVault

The preferred name is:

```text
MiniVault
```

MiniVault should eventually own:

- encrypted secrets,
- API keys,
- database credentials,
- application credentials,
- service accounts,
- service identities,
- credential bindings,
- expiration,
- rotation,
- revocation,
- audit history.

A later personal password-vault UI may be considered.

The primary purpose is ReactorLab infrastructure secrets and identity.

---

# 92. MiniVault Secret Model

Applications may conceptually bind values such as:

```text
vault://stripe/production
vault://myscheduler/jwt
```

The application receives the runtime value.

MiniAI sees metadata like:

```text
configured = yes
binding = healthy
expires = ...
last_rotated = ...
```

MiniAI should generally not see the plaintext value.

This must be structural, not prompt-based.

---

# 93. Phase 13 — MiniAI Agents

MiniAI should eventually become an orchestrator for specialized agents.

Likely agents:

```text
Investigation / Debugging Agent
Coding Agent
Deployment Agent
Database Agent
Security Agent
Recovery Agent
```

Agents must be separated by permissions and capabilities, not merely different prompts.

---

# 94. Coding Agent

Potential capabilities:

```text
Git branch writes
code editing
tests
preview deployment
```

Should not automatically have:

```text
production restart
plaintext secrets
arbitrary production DB mutation
```

---

# 95. Deployment Agent

Potential capabilities:

- deployment evidence,
- preview lifecycle,
- approved deployment actions.

Should not automatically have:

- unrestricted source modification,
- secret retrieval.

---

# 96. Recovery Agent

Potential capabilities:

- health evidence,
- recovery evidence,
- typed restart/recovery actions.

Should not have:

- generic shell,
- arbitrary Git writes,
- plaintext secrets.

---

# 97. Security Agent

Potential capabilities:

- security metadata,
- CVE research,
- configuration review,
- secret metadata,
- policy review.

Should not have plaintext secret dumping.

---

# 98. Why Agents Must Be Least-Privilege

A single omnipotent AI with every tool makes accidental cross-domain actions too easy.

Agent separation should improve:

- least privilege,
- auditability,
- specialization,
- safer reasoning,
- clearer authorization.

---

# 99. Phase 14 — Platform Backup / Disaster Recovery

Future backup scope should include:

- MiniBase databases,
- object storage,
- MiniVault encrypted state,
- MiniDeploy metadata,
- deployment identity,
- ReactorLab configuration,
- relevant audit/history state.

Goal:

```text
fresh machine
→ restore backup
→ recover ReactorLab cloud
```

---

# 100. Phase 15 — Installer / Hardware / Security Hardening

A finished installer should eventually detect/configure:

- CPU,
- RAM,
- storage,
- Docker,
- PostgreSQL,
- systemd,
- permissions,
- firewall/network defaults,
- Cloudflare integration,
- resource limits,
- inference capability,
- recovery/watchdog,
- update compatibility.

The exact installer technology is not yet fixed.

---

# 101. Phase 16 — Deployment Templates

Potential templates:

```text
React
Node/Express
Go
PostgreSQL-backed app
full-stack app
```

Templates should reduce repeated configuration without hiding what is actually deployed.

---

# 102. Phase 17 — Open-Source Productization

Before broad release:

- READMEs,
- architecture docs,
- security docs,
- installer,
- upgrade strategy,
- migration tests,
- examples,
- troubleshooting,
- CI,
- onboarding,
- reproducible setup.

The key transition is:

```text
works on Conrad's Dell
```

to:

```text
another developer can install and operate it
```

---

# 103. Phase 18 — Multi-Node ReactorLab

CURRENT DECISION:

```text
Multi-node IS part of the final roadmap.
```

Any older note saying multi-node was permanently removed is superseded.

Multi-node remains deliberately late.

---

# 104. Multi-Node Vision

Future node capabilities may include:

- node identity,
- secure enrollment,
- resource advertisement,
- CPU/RAM/storage capabilities,
- health,
- workload placement,
- node-aware services,
- cross-node networking,
- storage/database ownership,
- failure/recovery,
- centralized UI,
- MiniAI cloud-wide reasoning.

Instead of:

```text
the Dell
```

MiniAI should eventually reason:

```text
application
→ service
→ deployment
→ node
→ database
→ storage
→ network
→ metrics
→ recent changes
```

---

# 105. Multi-Node Non-Goals Initially

Do not immediately build:

- Kubernetes clone,
- complex consensus everywhere,
- service mesh,
- hyperscale scheduler,
- enormous control plane.

Start with the smallest safe distributed model that satisfies real ReactorLab needs.

---

# 106. Final User Experience

Ideal installation flow:

```text
fresh machine
   ↓
install ReactorLab
   ↓
hardware detected
   ↓
secure admin configured
   ↓
connect GitHub
   ↓
choose repository
   ↓
deploy
   ↓
optional DB/storage
   ↓
secret bindings
   ↓
domain
   ↓
health/logging
```

Then MiniAI becomes the operational interface.

---

# 107. Future Full Agentic Workflow

Example request:

```text
MyScheduler checkout is failing.
Find out why and fix it.
```

Potential safe workflow:

```text
MiniAI
   ↓
Investigation Agent
   ↓
typed platform evidence
   ↓
root cause
   ↓
Coding Agent
   ↓
Git branch
   ↓
source change
   ↓
tests
   ↓
preview deployment
   ↓
verification
   ↓
user review
   ↓
Deployment Agent
   ↓
authorized production deployment
```

This is one of ReactorLab's main long-term differentiators.

---

# 108. Hard Security Invariants

Do not introduce production MiniAI functionality equivalent to:

```text
arbitrary shell execution
generic command execution
generic privileged HTTP mutation
model-selected authoritative target IDs
plaintext secret dumping
authentication bypass
automatic retry of ambiguous mutations
confirmation automatically triggering execution
mutation during page reload
action execution solely because the model suggested it
```

Any proposal to introduce one of these requires explicit architecture review.

---

# 109. Evidence Invariants

MiniAI should:

- use typed evidence,
- retain target identity,
- retain timestamps,
- distinguish observation from inference,
- expose partial/missing coverage,
- fail closed for unsafe action eligibility,
- avoid absolute negative claims from incomplete evidence,
- keep software-owned confidence authoritative.

---

# 110. Action Invariants

Future mutations should normally contain:

```text
ActionKind
target kind
target ID
origin
safety class
policy
confirmation requirement
warning requirement
execution support
durable state
audit events
narrow executor
bounded failure categories
```

The model expresses intent.

Software creates authoritative actions.

---

# 111. Secret Invariants

```text
secret metadata
≠
secret plaintext
```

If a secret issue must be diagnosed, expose:

```text
configured
missing
expired
binding broken
rotation overdue
```

rather than the secret value.

---

# 112. Production Data Invariants

Do not delete or reset:

- MiniAI History,
- action proposals,
- action events,
- historical bug fixtures,
- MiniBase metadata,
- backups,

merely to make development or tests easier.

Historical failures are engineering evidence.

---

# 113. Development Workflow

Before substantial work:

```text
1. inspect current HEAD
2. inspect git status
3. inspect recent commits
4. inspect implementation
5. inspect tests
6. identify invariants
7. make coherent change
8. update tests
9. run formatting/tests
10. self-review diff
11. commit
12. deploy if required
13. live-test bounded fixture
14. update context/docs
```

---

# 114. Testing Expectations

For Go repositories, use relevant combinations of:

```text
gofmt

go test ./...

go test -count=1 ./...

go test -race ./...
when appropriate

go vet ./...

git diff --check
```

For frontend JavaScript:

- run existing frontend/Node tests,
- validate syntax,
- test relevant UI state,
- verify served assets.

Do not call a phase complete with known failing tests unless preserving an intentional failing reproduction.

---

# 115. Commit Style

Use Conventional Commits.

Examples:

```text
feat:
fix:
refactor:
test:
docs:
chore:
```

Prefer coherent milestone commits over noisy temporary commits.

---

# 116. Deployment Safety

When deploying a production ReactorLab service:

1. verify Git state,
2. verify intended commit,
3. run tests,
4. build candidate,
5. preserve prior binary/artifact if applicable,
6. install candidate,
7. restart only required service,
8. inspect systemd status,
9. verify health,
10. verify storage/database integrity where relevant,
11. inspect logs,
12. perform bounded live regression.

Do not use full Dell reboot as routine deployment procedure.

---

# 117. Canary Policy

Mutation testing should prefer:

```text
miniai-restart-canary
```

rather than:

```text
MyScheduler
Golf Mullet
other real applications
```

The canary exists specifically for controlled destructive/restart testing.

---

# 118. Production Authentication

Cloudflare Access is part of the current administrator security boundary.

Do not weaken authentication to make automated testing easier.

If automation requires admin access:

- use an authenticated browser/session,
- or design a proper safe machine authentication mechanism later.

Do not add local bypasses.

---

# 119. Known Service Origins

Known current/historical service origins include:

```text
MiniDeploy
127.0.0.1:9000

MiniBase private
127.0.0.1:9100

MiniBase public origin
127.0.0.1:9103

MiniAI
127.0.0.1:9300
```

Always verify before relying on them.

---

# 120. Why Cloudflare Is Used

Cloudflare currently helps provide:

- secure public routing,
- stable subdomains,
- Access authentication,
- reduced need for broad inbound server exposure.

Origin services still validate important security properties.

Do not assume reverse proxy protection eliminates the need for origin checks.

---

# 121. Public vs Admin Surface

A public service may intentionally expose:

```text
health
Guest/public information
application traffic
```

while protecting:

```text
management
mutation
internal state
secrets
```

Public hostname does not mean public management API.

---

# 122. Error Sanitization

Public/API errors should avoid exposing unnecessary:

- local filesystem paths,
- raw SQL,
- credentials,
- internal implementation details.

Logs can retain deeper diagnostics when appropriate.

---

# 123. Auditability

A future ReactorLab action should be able to answer:

```text
Who suggested this?

Which investigation produced it?

Which evidence supported it?

Who authorized it?

When did execution begin?

Which executor ran?

What was the outcome?

Was the outcome ambiguous?

What was observed afterward?
```

Auditability is a product feature, not merely debugging infrastructure.

---

# 124. Why Phase 4 Matters to the Future

Self-healing and future agents should reuse the Phase 4 architecture.

Do not invent different semantics for:

- Recovery Agent,
- Deployment Agent,
- Database Agent,
- automated recovery,
- multi-node actions.

Reuse:

```text
typed actions
policy
confirmation
execution
audit
failure semantics
reconciliation
```

---

# 125. Current Resume-Level Final Vision

Only when corresponding features actually exist, ReactorLab should eventually be describable as:

> An open-source, secure, multi-node AI-operated personal developer cloud with application deployment, managed databases and storage, encrypted secrets, specialized infrastructure agents, evidence-grounded incident investigation, preview environments, controlled source modification, self-healing, disaster recovery, and distributed workload orchestration on commodity hardware.

Do not represent roadmap features as completed features.

---

# 126. Codex Workspace Model

Codex should treat the project as a multi-repository platform workspace.

Current expected source folders:

```text
ReactorLab
MiniDeploy
MiniBase
MiniAI
miniai-restart-canary
```

Future repositories should be added when created.

Do not treat MiniAI as an isolated application when platform-level changes require multiple repositories.

At the same time, do not modify unrelated repositories merely because they are present.

---

# 127. Codex Autonomy Philosophy

The desired workflow is:

> High autonomy within a user-authorized phase, explicit human review between phases.

The purpose is to remove repetitive:

```text
ChatGPT prompt
→ copy into Codex
→ terminal command
→ copy output
→ next prompt
```

while preserving architectural oversight.

Codex should increasingly own entire milestones end-to-end.

---

# 128. Absolute Roadmap Phase Ceiling

Every substantial Codex task must have an explicit authorized roadmap ceiling.

Example:

```text
AUTHORIZED ROADMAP CEILING: Phase 4D1
```

This means Codex may fully complete everything legitimately required for 4D1.

It must NOT implement:

```text
4D2
4D3
Phase 5
or anything later
```

unless the user explicitly raises the ceiling.

---

# 129. Completion Does Not Grant More Authority

If Codex finishes the authorized phase early:

```text
unused time
unused context
unused usage
unused compute
```

does NOT authorize it to begin the next phase.

Only the user can expand roadmap scope.

---

# 130. No Opportunistic Roadmap Advancement

Codex must not silently include future features because they are nearby or convenient.

Examples:

While doing 4D1:

```text
DO NOT add second executable action.
```

While doing previews:

```text
DO NOT start source editing.
```

While doing MiniVault:

```text
DO NOT give MiniAI plaintext secret access.
```

While doing single-node work:

```text
DO NOT begin multi-node architecture without authorization.
```

Necessary refactoring for the current milestone is allowed.

Future feature implementation is not.

---

# 131. Fully Complete the Authorized Phase

A phase ceiling does not mean:

```text
do the smallest possible patch
```

It means:

```text
fully complete this milestone
then stop
```

If acceptance criteria require:

- backend,
- frontend,
- tests,
- migration safety,
- live deployment,
- canary proof,
- documentation,
- security validation,

those all remain within authorized scope.

---

# 132. End-of-Phase Stop Rule

At completion of the authorized phase, Codex must STOP implementation.

It should report:

1. what was implemented,
2. repositories changed,
3. important files changed,
4. tests run,
5. test results,
6. deployment performed,
7. production/live verification,
8. commits created,
9. known limitations,
10. unresolved technical debt,
11. acceptance criteria status,
12. what the next phase is,
13. what that next phase would accomplish.

It may recommend the next phase.

It may not start it.

---

# 133. Codex Usage Budget

The user wants to preserve Codex allowance rather than accidentally consume the entire available budget in one autonomous run.

Usage efficiency is therefore a development constraint.

Codex should avoid unnecessary:

- repeated repository scans,
- repeated rereading of large unchanged files,
- full-suite test runs after every tiny edit,
- speculative rewrites,
- repeated documentation regeneration,
- repeated live-production polling,
- redundant model reasoning,
- unrelated repository investigation.

---

# 134. Usage-Efficient Development Pattern

During implementation:

```text
inspect once
→ targeted changes
→ targeted tests during iteration
→ full required suite near completion
→ one self-review
→ bounded production verification
```

Do not repeatedly run expensive full-system checks when a smaller test answers the immediate question.

---

# 135. Usage Reserve Rule

When the user supplies a usage reserve, treat it as a work-budget boundary.

Preferred wording:

```text
MINIMUM REMAINING CODEX ALLOWANCE: X%
```

Example:

```text
MINIMUM REMAINING CODEX ALLOWANCE: 30%
```

This is preferred over:

```text
use no more than 70%
```

because "remaining" is less ambiguous.

---

# 136. Usage Visibility Limitation

Codex must only enforce an exact usage percentage if it has reliable access to the relevant current usage information.

If it cannot reliably observe the user's allowance:

```text
DO NOT invent a percentage.
DO NOT estimate one.
DO NOT claim it stopped at a threshold it cannot see.
```

Instead it should:

- work efficiently,
- stop at natural safe boundaries if the budget may be at risk,
- tell the user when manual usage review is required before a large remaining step.

---

# 137. Multiple Usage Windows

If multiple allowance windows exist and are visible, the most constrained relevant allowance should normally control the budget unless the user specifies otherwise.

Do not assume one percentage represents every usage limit.

---

# 138. No Automatic Spending

Codex must never automatically:

- purchase credits,
- purchase extra usage,
- upgrade the plan,
- redeem paid resets,
- incur charges,

merely to continue ReactorLab development.

Running out of included usage means STOP unless the user explicitly authorizes otherwise.

---

# 139. Recommended Per-Phase Task Header

Every major task should begin with something equivalent to:

```text
REACTORLAB EXECUTION BOUNDARY

AUTHORIZED ROADMAP CEILING: Phase X

MINIMUM REMAINING CODEX ALLOWANCE: Y%

Complete Phase X end-to-end according to REACTORLAB_CONTEXT.md
and the current repositories.

Do not implement any later phase.

When the authorized phase meets its acceptance criteria:
STOP and provide the phase-completion report.

If the usage reserve can be observed reliably:
stop before crossing it.

If it cannot:
do not fabricate usage information.

Do not purchase additional usage.
```

---

# 140. User Review Between Phases

The intended development loop is:

```text
User + ChatGPT
define/review milestone
        ↓
Codex
owns milestone end-to-end
        ↓
Codex STOP
        ↓
User + ChatGPT
review implementation and architecture
        ↓
explicitly authorize next milestone
```

This loop should remain even when Codex becomes capable of doing much more autonomously.

---

# 141. When Codex Should Ask/Stop

Codex should stop for input when genuinely necessary, particularly:

- credentials,
- browser authentication it cannot complete,
- destructive or irreversible production operations,
- major architecture forks,
- uncertain production state,
- financial actions,
- externally consequential actions,
- ambiguity where a wrong decision risks real data or infrastructure.

---

# 142. When Codex Should NOT Stop

Do not stop merely because:

- a normal engineering choice is required,
- a test fails and needs debugging,
- several files need refactoring,
- a bounded implementation detail was not specified,
- relevant code must be inspected,
- a phase requires multiple iterations.

Use existing architecture and engineering judgment.

The goal is autonomous milestone completion, not constant permission requests.

---

# 143. Context Maintenance

At the end of a completed phase, update this context only with durable facts.

Add:

- completed phase,
- important commit,
- important architectural changes,
- new invariants,
- live proof,
- production fixtures,
- known limitations,
- next milestone.

Do not turn this document into a minute-by-minute diary.

---

# 144. Future Documentation Split

As ReactorLab grows, machine-verified implementation details should increasingly move into specialized documents such as:

```text
ARCHITECTURE.md
ROADMAP.md
SECURITY.md
PRODUCTION.md
API_CONTRACTS.md
DATA_MODELS.md
OPERATIONS.md
TESTING.md
DECISIONS.md
AGENTS.md
```

This context should remain the master narrative and handoff document.

Do not duplicate huge amounts of volatile generated implementation detail here if a dedicated verified document is better.

---

# 145. Architecture Decision Records

Important decisions should eventually be formalized as ADRs.

Initial ADR candidates:

```text
ADR-001
MiniAI receives no arbitrary shell capability.

ADR-002
Application lifecycle mutation belongs to MiniDeploy.

ADR-003
Managed data belongs to MiniBase.

ADR-004
Secrets and service identity belong to MiniVault.

ADR-005
Model output is not authoritative infrastructure identity.

ADR-006
Confirmation and execution are separate transitions.

ADR-007
Ambiguous mutations are not automatically retried.

ADR-008
Current evidence is required for restart eligibility.

ADR-009
History and action audit records are durable production evidence.

ADR-010
Source editing is Git-backed.

ADR-011
Preview environments precede autonomous source editing.

ADR-012
Agents use least privilege.

ADR-013
Single-node maturity precedes multi-node expansion.

ADR-014
Multi-node remains part of the final ReactorLab vision.
```

---

# 146. Current Repository Baselines

Snapshot at this context update:

```text
ReactorLab
b456264543c06ee84b088fd51433ca16461a2b28
feat: add MiniAI universal read contract

MiniDeploy
c4e03e98be3d9811503c4dbbc38f417e131c6b65
feat: add exact deployment source identity

MiniBase
05ced46bd60b31ba87cac852c985717465694fc2
chore: reorder MiniBase admin navigation

MiniAI
b55e4a2e7ac0bd53e89da1cb50b6bf80f164799b
fix: harden assistant restart eligibility

miniai-restart-canary
d0ac280eb2af3e5f8d9069c0143b43bc2df23751
chore: add MiniAI restart canary
```

VERIFY ALL OF THESE BEFORE WORK.

---

# 147. Current Roadmap

Completed foundation:

```text
MiniDeploy deployment/runtime foundation
MiniBase database foundation
ReactorLab control-plane foundation
ReactorLab recovery system
MiniAI typed read evidence
MiniAI bounded investigation
MiniAI durable History
MiniAI typed action policy
MiniAI durable actions
MiniAI admin auth
MiniAI restart executor
MiniAI controlled execution
assistant restart recommendations
semantic restart hardening
interactive action UI
live confirmation/execution proof
reload safety proof
```

Next:

```text
Phase 4D1
Action failure / ambiguity / reconciliation hardening

Phase 4D2
Action observability / recovery UX

Phase 4D3
Second real typed action
```

Then:

```text
Phase 5
Preview deployments

Phase 6
Controlled source editing

Phase 7
Controlled web research

Phase 8
Evaluation / hardening

Phase 9
Log Explorer / observability

Phase 10
Self-healing / automation

Phase 11
MiniBase object storage

Phase 12
MiniVault

Phase 13
MiniAI specialized agents

Phase 14
Platform backup / disaster recovery

Phase 15
Installer / hardware detection / security hardening

Phase 16
Deployment templates

Phase 17
Open-source productization

Phase 18
Multi-node ReactorLab

Final stabilization
```

Numbering may evolve as requirements become clearer.

Do not treat numbering as more important than architecture.

---

# 148. Immediate Current Instruction

At the time this document was created:

```text
Phase 4C is complete.
```

Current next milestone:

```text
Phase 4D1
```

Codex should first:

1. inspect all workspace repositories,
2. inspect Git status in all repositories,
3. inspect current HEADs,
4. compare implementation with this context,
5. report any discrepancies,
6. verify that Phase 4D1 is still the correct next milestone.

Do not modify implementation code during the initial audit unless the user explicitly asks to begin the phase.

---

# 149. Definition of Done for a Phase

A phase is not complete simply because:

```text
code compiles
```

A phase is complete when its meaningful acceptance criteria have passed.

Depending on phase, this may include:

- implementation,
- unit tests,
- integration tests,
- race/vet checks,
- frontend tests,
- migrations,
- deployment,
- production health,
- live canary test,
- security validation,
- documentation,
- self-review,
- Git cleanliness.

---

# 150. Final Decision Priority

When an engineering decision is ambiguous, optimize approximately in this order:

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

Performance matters.

Automation matters.

AI capability matters.

But none should override a broken trust boundary.

---

# 151. Final ReactorLab Principle

The defining principle of the entire project is:

> Make ReactorLab powerful enough for AI operation while keeping authority in deterministic software.

The AI may eventually become extremely capable.

It may investigate incidents.

It may coordinate specialized agents.

It may write code.

It may build previews.

It may recommend recovery.

It may deploy approved changes.

It may operate across multiple physical nodes.

But the platform must continue to ensure:

```text
AI intelligence
≠
unrestricted authority
```

That distinction is what should make ReactorLab both useful and trustworthy.

---

# 152. Phase 4D1 Candidate Checkpoint — November 17, 2026

The documentation baseline `17e2c91c18ee3cfdb166d8904f0844b2d26aeea5` was
pushed to `origin/main` before implementation.

MiniAI candidate commit:

```text
87cd2968fefe5931e257ae6686acde4ccec24927
fix: harden action failure and reconciliation
```

Phase 4D1 is implemented locally but is NOT COMPLETE. Production deployment
and live acceptance remain pending administrator authentication. The Dell was
checked read-only and still runs the prior `b55e4a2` baseline. Sudo requires a
password. A verified candidate and guarded backup/install helpers are staged
privately under `/tmp/miniai-phase4d1-87cd296`; no service restart, production
database migration, or live canary mutation has occurred at this checkpoint.

Durable design facts in the candidate:

- SQLite schema v4 adds `execution_outcome_unknown` while preserving existing
  proposals and event values. A migration is needed because v3 CHECK constraints
  cannot represent the event.
- Unknown execution remains lifecycle state `executing`; the API and UI report
  `outcome_unknown`, including legacy interrupted dispatches.
- Execute, GET, and History share the same audit-derived outcome, safe failure
  category, and execution timestamps.
- Refresh is GET-only and writes no events. Current target state never proves
  restart causation. There is no automatic retry or new executor.
- Unclassified executor errors formerly claimed definitive failure; this direct
  prerequisite defect now conservatively yields unknown, with regression coverage.
- The old v3 binary cannot open a v4 database. Preserve a consistent database
  backup and prior binary before installation; do not overwrite later audit
  activity during any reviewed rollback.

Local validation passed: full MiniAI tests, race tests, vet, eight frontend
tests, JavaScript syntax checks, whitespace checks, and Linux amd64 cross-build.
Tests include a real local post-dispatch disconnect, migration preservation,
reopen/History parity, actual served UI restoration, GET-only Refresh, and
duplicate-execution rejection. Other implementation repositories are unchanged.

The authorized ceiling remains Phase 4D1. Phase 4D2 has not started.

## Superseding overnight execution boundary

The user subsequently authorized local Phase 4D1 work only and explicitly
required stopping before deployment, binary replacement, service restart,
live action/canary mutation, or any other production write. This supersedes the
earlier request to execute the staged deployment helper. Do not run it until
the user authorizes resuming production verification.

The clean MiniAI candidate `87cd2968fefe5931e257ae6686acde4ccec24927` is:

```text
READY FOR PRODUCTION VERIFICATION
```

No implementation changed after its passing local verification. The exact
remaining production steps are recorded in `PRODUCTION_VERIFICATION.md`.
Phase 4D1 is not yet complete. Phase 4D2 remains unstarted.
