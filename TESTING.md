# ReactorLab Testing

Status: audit results and expectations verified November 17, 2026.

[`REACTORLAB_CONTEXT.md`](REACTORLAB_CONTEXT.md) remains the master narrative.
This file does not claim production or live-Dell verification.

## Audit test results

| Repository/suite | Result |
|---|---|
| MiniAI `go test ./...` | Passed |
| MiniAI `node --test frontend_action_controls_test.js` | Passed: 4 tests |
| ReactorLab `go test ./...` | Failed: 3 API tests on macOS because `/proc/uptime` is unavailable |
| MiniDeploy `go test ./...` | Failed: 1 planning test because lexical `/var/...` differs from resolved `/private/var/...` on macOS |
| MiniBase `go test ./...` | Build failed on macOS: `unix.Renameat2` and `unix.RENAME_NOREPLACE` are unavailable on Darwin |
| miniai-restart-canary | No automated harness; deliberately static |
| ReactorLab/MiniDeploy/MiniBase frontend suites | Not run: `node_modules` was absent and the audit did not install dependencies |

These are development-platform portability findings, not evidence that the
Linux production behavior is broken. They must not be described as passing.

## Current test expectations

For Go repositories, use targeted tests while iterating and the complete
required set near milestone completion:

```text
gofmt
go test ./...
go test -count=1 ./...
go test -race ./...       when appropriate
go vet ./...
git diff --check
```

For React/Vite repositories:

```text
npm ci
npm test -- --run
npm run lint
npm run build
```

MiniAI action-control test:

```text
node --test frontend_action_controls_test.js
```

Do not install dependencies or change lockfiles merely to make an audit appear
complete. Record what was and was not run.

## Repository coverage

- ReactorLab: authentication, APIs, upstream validation, observability,
  recovery, metrics, relationships, and UI. Linux host collectors need Linux
  execution or injected fixtures.
- MiniDeploy: strategies, lifecycle, bindings, source identity, security,
  visibility, observability, rollback, and failure protection. Path assertions
  must account for macOS symlink resolution.
- MiniBase: metadata, provisioning, backup/restore, deletion, attachments,
  Access, Guest isolation, exploration, and UI. Linux atomic filesystem calls
  currently block the full Mac build.
- MiniAI: routing, evidence, validation, confidence, History, auth, action
  persistence/execution, suggestions, UI controls, and power lease. Audited Go
  and action-control suites pass on the Mac.
- Canary: static content, no data/secrets, observable stop/restart, unchanged
  container identity for restart proof, and bounded HTTP health.

## Production and live testing

No production access, deployment, restart, or canary mutation occurred during
the documentation audit. Live tests require explicit authorization, must prefer
the canary, and must not weaken Cloudflare Access or delete historical records.

## Phase 4D1 candidate verification

MiniAI candidate `87cd2968fefe5931e257ae6686acde4ccec24927` passed:

- `go test -count=1 ./...`;
- `go test -race ./...`;
- `go vet ./...`;
- `node --test frontend_action_controls_test.js frontend_action_app_test.js`
  (8 tests);
- JavaScript syntax checks and `git diff --check`;
- Linux amd64 cross-build from the clean committed source.

New coverage includes real local HTTP disconnect after dispatch, durable
ambiguity/failure, Execute/GET/History parity, reopen, read-only reload/Refresh,
completion races, duplicate execution, preserved v3 fixture identities and row
hashes, and actual served frontend code exercised with a small DOM adapter.

Production acceptance remains pending. Read-only Dell preflight confirmed the
expected prior commit, healthy MiniAI, and a running canary. No deployment or
canary mutation has occurred at this checkpoint; sudo requires user authentication.

The overnight boundary subsequently required stopping before all production
writes. The unchanged candidate is READY FOR PRODUCTION VERIFICATION; this does
not mark live acceptance complete. See
[`PRODUCTION_VERIFICATION.md`](PRODUCTION_VERIFICATION.md) for the remaining
steps. Previously passing suites were not repeated without a code change.
