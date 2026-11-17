# ReactorLab Operations

Status: repository-verified operational reference as of November 17, 2026.

This is not proof of current live production state. Verify the Dell before
relying on a path, port, service status, timer, container identity, or deployed
commit. No production access occurred during this documentation audit.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) remains the master narrative.

## Current verified configuration

Repository-configured origins:

| Service | Private origin | Public origin where applicable |
|---|---:|---:|
| MiniDeploy | `127.0.0.1:9000` | `127.0.0.1:9003` |
| MiniBase | `127.0.0.1:9100` | `127.0.0.1:9103` |
| ReactorLab | `127.0.0.1:9200` | configured public routing |
| MiniAI | `127.0.0.1:9300` | configured administrator routing |
| Ollama | `127.0.0.1:11434` | none |

Known production locations include:

```text
/srv/reactorlab
/srv/minibase
/srv/miniai
/var/lib/miniai/miniai.db
```

Other paths must be verified. The Dell is production infrastructure, not a
disposable test runner.

## Ownership and deployment

- Restart applications through MiniDeploy, not ad hoc Docker commands.
- Operate databases/backups through MiniBase lifecycle APIs.
- Preserve ReactorLab recovery/observability and MiniAI History/action audit.
- Prefer `miniai-restart-canary` for restart mutation tests.

Authorized production deployment procedure:

1. Verify status, branch, and intended commit.
2. Run relevant tests and formatting.
3. Build the candidate and preserve the prior artifact when applicable.
4. Install and restart only the intended service.
5. Check systemd, health, bounded logs, and required storage integrity.
6. Run a bounded fixture regression and record evidence.

A full Dell reboot is not a routine deployment mechanism.

## Database and backup operations

MiniBase supports manual/automatic backup, restore-as-new, replace-current,
retention, and deletion safety. The repository states that automatic backups
run daily at 03:00 UTC using a persistent timer; verify the installed timer.

Same-Dell backups protect against logical/application failure, not SSD loss.
Application rollback must not restore database contents. Attached database
deletion is blocked, and partial cleanup remains retryable.

## MiniAI action operations

```text
investigate -> propose -> confirm -> separately execute -> read state
```

On conflict, network ambiguity, no status, server error, or unexpected response,
the browser reconciles with GET and does not resend mutation. Duplicate Execute
requests are rejected by persisted state.

The deployed Phase 4D1 implementation preserves `outcome_unknown`, safe
categories, and audit timestamps through GET/reload. The lifecycle state remains `executing` until an
authoritative result exists. Observing an application running does not prove the
restart caused it. GET refresh performs no writes or redispatch.

The user explicitly resumed production verification after the overnight stop
and ran the guarded sudo deployment. The deployed implementation is MiniAI
commit `87cd2968fefe5931e257ae6686acde4ccec24927`. Schema v4 migration and health
checks passed. The consistent pre-deployment database backup, old binary, and
historical hashes are retained under `/var/lib/miniai/phase4d1-87cd296`.
After migration, do not attempt a binary-only downgrade. Do not restore an old
backup over later History/action activity without preserving that activity.

The disposable canary is running and healthy after one typed direct-user restart.
Do not rerun the deployment helper or acceptance mutations merely to reproduce
the report. Results and the remaining assistant-suggestion acceptance limitation
are in [`PRODUCTION_VERIFICATION.md`](PRODUCTION_VERIFICATION.md).

## Recovery

ReactorLab records host recovery incidents and current watchdog/RTC protection.
Recovery notifications use a durable outbox. Historical outage, recovery, and
notification records must not be deleted for convenience.

## Development-machine limitations

- ReactorLab tests instantiate Linux `/proc` collectors on the Mac.
- MiniDeploy has a macOS `/var` versus `/private/var` assertion mismatch.
- MiniBase uses Linux `renameat2` without a Darwin build split.

See [`TESTING.md`](TESTING.md).

## Future operations

Not implemented: off-Dell recovery,
MiniVault rotation/revocation, installer/upgrade automation, or multi-node
placement and recovery.
