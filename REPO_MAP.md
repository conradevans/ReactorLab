# ReactorLab Repository Map

Status: verified workspace map as of November 17, 2026.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) remains the master narrative.

## Current verified repositories

| Repository | Verified HEAD | Role |
|---|---|---|
| `ReactorLab` | `b456264543c06ee84b088fd51433ca16461a2b28` | Umbrella platform, observability, recovery, and unified control plane |
| `MiniDeploy` | `c4e03e98be3d9811503c4dbbc38f417e131c6b65` | Application deployment and runtime |
| `MiniBase` | `05ced46bd60b31ba87cac852c985717465694fc2` | Managed PostgreSQL and backups |
| `MiniAI` | `b55e4a2e7ac0bd53e89da1cb50b6bf80f164799b` | Evidence-grounded assistant and action control plane |
| `miniai-restart-canary` | `d0ac280eb2af3e5f8d9069c0143b43bc2df23751` | Disposable restart fixture |

At audit time all repositories were on `main`, tracking `origin/main`, with
no reported divergence. They were clean except for the newly created untracked
`ReactorLab/REACTORLAB_CONTEXT.md`.

## Current dependency map

```text
ReactorLab
  -> reads MiniDeploy and MiniBase
  -> probes MiniAI health
  -> exposes typed GET-only evidence to MiniAI

MiniDeploy
  -> owns application/container mutations
  -> calls MiniBase's private database-binding API

MiniBase
  -> owns database and backup lifecycle
  -> calls MiniDeploy for attach/detach coordination

MiniAI
  -> reads ReactorLab typed evidence
  -> reads bounded MiniDeploy logs where no ReactorLab proxy exists
  -> calls MiniDeploy's narrow restart endpoint

miniai-restart-canary
  -> is hosted by MiniDeploy and targeted by bounded MiniAI restart tests
```

## Change ownership

| Change | Owner | Common collaborator |
|---|---|---|
| Deployment, container, restart, rollback, domain, preview | MiniDeploy | ReactorLab or MiniAI when contracts change |
| Database, backup, restore, attachment, deletion | MiniBase | MiniDeploy for bindings; ReactorLab for evidence |
| Host recovery, aggregation, historical telemetry | ReactorLab | MiniDeploy and MiniBase as sources |
| Reasoning, confidence, History, action policy/UI/audit | MiniAI | ReactorLab for reads; MiniDeploy for execution |
| Restart regression fixture | miniai-restart-canary | MiniDeploy and MiniAI |
| Platform narrative, decisions, and roadmap | ReactorLab | All repositories are sources |

Repository presence does not authorize unrelated edits.

## Phase 4D1 impact map

Phase 4D1 has not begun. MiniAI is the primary repository. MiniDeploy is the
authoritative restart dependency and should change only if the current contract
cannot safely satisfy the milestone. ReactorLab evidence may support
current-state observation, but no change is yet proven necessary. The canary is
the live fixture. MiniBase is outside the milestone.

## Future repositories

`MiniVault` is the selected future secrets and service-identity component. It
does not currently exist.
