# Phase 4D1 Production Verification

Status: PARTIAL production acceptance, November 17, 2026. The implementation is
deployed and the canary is healthy. Phase 4D1 is not marked complete; Phase 4D2
has not started.

## Current execution boundary

The user explicitly resumed bounded production verification after running the
password-protected deployment helper. This supersedes the earlier overnight
stop. No authentication was weakened. Bounded mutations are now finished; do
not repeat the reference procedure below just to reproduce this report.

## Current verified production results

- Dell checkout and installed binary matched implementation commit `87cd296`
  and the SHA-256 below. MiniAI remained active with PID 619905 and healthy.
- Schema migrated from v3 to v4. The user's deployment output confirmed SQLite
  integrity/foreign-key checks and every pre-existing proposal/event/History row
  hash were preserved, including all five historical action fixtures. Baseline
  counts at deployment were 5 proposals / 15 events / 23 History entries.
- Unauthenticated action GET returned 401; authenticated GET used `no-store`.
- The missing target was independently verified absent before execution.
  `action_34837674e3c5a8c38a90c852484419e3` became `failed`, with outcome
  `failed`, category `upstream_rejected`, start `19:48:30.672077419Z`, completion
  `19:48:30.673307051Z`, and three ordered events (policy, start, failure).
  The public Execute request returned 502 with HTML rather than origin JSON;
  GET recovered the durable result. A duplicate Execute returned 409 and changed
  no action/event values. The real canary remained running and unchanged during
  this test.
- The canary was then stopped once for the assistant-flow regression. Four
  investigations created no action: two validated-answer rejections, one
  legacy/generic truncated answer, and one canceled network stream. No unsafe
  proposal was fabricated and the evidence validator was not relaxed.
- To leave the fixture healthy and verify the deployed success path, one normal
  direct-user action was created: `action_9bf4f2176b9315bce25d727e2836f953`.
  Creation left the canary stopped. Separate Execute returned 200 / `succeeded`,
  start `19:59:32.014636274Z`, completion `19:59:32.168783111Z`, and three events
  (policy, start, success). Execute and GET were identical. A duplicate returned
  409 without changing events.
- The same container `be428f2dc3b843caa99e74ae7552871f4091a9ea9c4b5ac76e673411939b6204`
  was running with new start `2026-11-17T19:59:32.047913331Z`; its actual loopback
  port 8082 returned the static `MiniAI Restart Canary` / `Healthy.` page.
- API History contained 27 entries with no further page: precisely four new
  investigations. All four had zero actions. The two explicit test actions add
  six events; expected final table totals are therefore 7 proposals / 21 events /
  27 History entries, subject to the protected-database final check below.
- Historical action/event GET snapshots matched exactly after the live tests.
  No historical fixture was deleted or normalized.
- Both new test records were opened and reloaded in the browser. Subsequent
  authenticated GET comparisons showed identical action values and event chains;
  the canary's start timestamp did not change.

## Remaining acceptance and explicit limitations

1. A fresh assistant suggestion -> Confirm only (still stopped) -> separate
   Execute -> successful UI reload live sequence remains unproven. Automated
   coverage and earlier Phase 4C historical evidence are intact, but the live
   direct-user test is not a substitute for this exact sequence. Resolve or
   explicitly review the model/stream reliability limitation before marking
   Phase 4D1 complete. Do not silently broaden the phase into a reasoner redesign.
2. The final protected-database read-only hash/integrity check was requested from
   the user because sudo requires their password. Deployment-time full hashes
   passed; subsequent API comparisons cover all five existing actions/events.
   Do not claim a second full database hash check until its output is received:

   ```sh
   ssh -t mini-server 'sudo python3 /tmp/miniai-phase4d1-87cd296/audit.py after'
   ```

Ambiguity was intentionally not induced live: there is no safe deterministic
production fixture, and disrupting MiniDeploy/networking would create needless
risk. Passing local integration tests cover real post-dispatch response loss,
database reopen, GET/History/UI restoration, no repeated POST, and no inference
of causation from a running target. Unknown may remain unknown permanently.

## Verified candidate

```text
Repository: MiniAI
Commit: 87cd2968fefe5931e257ae6686acde4ccec24927
Message: fix: harden action failure and reconciliation
Platform: linux/amd64, CGO_ENABLED=0
Binary SHA-256:
960406fa507f5aaf046e4d44313231dac11104896cf0bb1af76d4ba6f8565b76
```

Local validation passed against this implementation:

- full MiniAI `go test -count=1 ./...`;
- full MiniAI `go test -race ./...`;
- `go vet ./...`;
- `node --test frontend_action_controls_test.js frontend_action_app_test.js`
  (8 tests at implementation commit; 9 after the gateway-error regression);
- JavaScript syntax checks and `git diff --check`;
- Linux cross-build with the candidate revision and `vcs.modified=false`.

Self-review covered dispatch count, confirmation/execution separation, GET-only
refresh, authentication, target binding, restoration, concurrent completion,
safe error serialization, migration preservation, and the unchanged closed
executor catalog. An unclassified executor error no longer falsely establishes
definitive failure; it conservatively records unknown.

## Reference procedure from the pre-deployment handoff

This records the original procedure, not an instruction to repeat completed
deployment or mutation steps. Current results and the actual remaining work
are above. The implementation binary has not changed after deployment; the
follow-up change adds only a frontend regression test and documentation.

### 1. Revalidate the deployment target

After fresh authorization, verify SSH identity, the current MiniAI repository
HEAD/status, the running service binary, and health. The last read-only preflight
observed clean `/srv/miniai` at `b55e4a2`, an active service, and the running
canary. These are historical observations, not a fresh production guarantee.

Check for active investigations before stopping the service. Use ordinary
administrator authentication; neither localhost nor Docker-group membership is
a substitute for access to the MiniAI administrator API or service-owned data.

The candidate, source bundle, and backup/install helpers were staged before the
overnight boundary under `/tmp/miniai-phase4d1-87cd296`. Verify their contents,
hashes, and availability; temporary staging may not survive a reboot. The local
candidate and helpers also existed under `/tmp` on the Mac. Rebuild from the
clean pinned commit if necessary.

### 2. Preserve data and install only MiniAI

1. Verify the candidate binary hash and source commit.
2. Stop only `miniai.service` during an approved quiet window.
3. Create a consistent SQLite backup using SQLite's backup API, not a copy of
   only the live main database file. Preserve the old binary as well.
4. Record schema version, integrity/foreign-key checks, proposal/event/History
   counts, and hashes of existing rows. Keep snapshots private and avoid
   printing request text, credentials, or raw private data.
5. Confirm all five historical action fixtures listed below are present.
6. Advance the clean Dell checkout to the pinned candidate and install its
   binary atomically, preserving existing service/authentication configuration.
7. Start only MiniAI. Verify systemd status, `GET /health`, the running binary,
   and bounded recent logs without exposing secrets.

The staged helper is designed to perform this sequence, but must be reviewed
again before use if production state or its contents changed.

### 3. Verify migration and authentication

Confirm schema version 4, SQLite integrity, foreign-key integrity, and unchanged
pre-existing proposal/event/History hashes. Check existing successful and
cancelled action records through authenticated GET. Verify unauthenticated
administrator requests remain rejected.

The v3 binary refuses a v4 database. A binary-only downgrade is insufficient.
Do not restore a pre-deployment database over new History/action activity. If
recovery is necessary, first preserve the current state and review a plan that
accounts for every new record.

### 4. Prove the normal canary path

1. Read the canary's exact container ID, status, and start timestamp.
2. Stop only `miniai-restart-canary` in the authorized live-test window.
3. Ask MiniAI to investigate the stopped canary and obtain one assistant restart
   proposal bound to that exact application.
4. Confirm through the authenticated UI. Verify the action is `authorized`, the
   canary remains stopped, and no execution-start event exists yet.
5. Separately Execute once. Verify `succeeded`, `execution_outcome=succeeded`,
   execution timestamps, and the ordered audit chain ending in
   `execution_succeeded`.
6. Verify the same container ID is running and HTTP returns the canary's healthy
   response. This establishes restart rather than replacement deployment.
7. Reload the page and read the action/History. The successful outcome and
   timestamps must persist without a new event or restart.
8. Make one duplicate Execute request for the same action using an authenticated
   client. Expect conflict and no additional restart/event.

### 5. Prove definitive pre-mutation failure safely

Use an absent target rather than altering a real application. First verify
`miniai-restart-canary-missing-4d1` is absent in MiniDeploy. With an authenticated
administrator client, create exactly one direct-user proposal using:

```text
POST /api/admin/actions
{"kind":"restart_application","target":{"type":"application","id":"miniai-restart-canary-missing-4d1"},"arguments":{}}
```

Execute that proposal once. Expect MiniDeploy's definitive 404 to produce a
502 Execute response with `state=failed`, `execution_outcome=failed`, and
`failure_category=upstream_rejected`. GET must preserve those fields and
timestamps, with `execution_started` followed by `execution_failed`.

Read the event chain repeatedly and make one duplicate Execute attempt; expect
409 and no second dispatch. Check the real canary remains unchanged. The UI's
failure rendering and History restoration already have automated coverage;
verify visually where the supported UI exposes the tested record. Do not
fabricate a History link or modify production rows to manufacture a card.

If an authenticated client is unavailable, report the live proof as unperformed.
Do not extract credentials through an unsupported path or weaken authentication.

### 6. Keep ambiguous production testing bounded

Do not disrupt MiniDeploy or the network to create ambiguity. Unless a safe,
deterministic fixture is separately available, retain the passing integration
proof: a local HTTP server accepts restart, loses its response, and the outcome
survives database reopen, GET, History, and frontend restoration without another
POST. A running target cannot relabel unknown as succeeded.

Explicitly report that production ambiguity was not deliberately induced. An
unknown action may remain unknown permanently; current application state is not
an execution receipt.

### 7. Preserve the audit and finish the phase

Compare final counts and row hashes with the pre-deployment snapshot. Account
for only the new test investigations, proposals, and events. Preserve all
historical rows and leave the canary healthy. Required historical fixtures:

```text
action_6b29ccf5ff1eb4312a2c5b6769be8e90
action_ec81dad4db3e2a1d536c326bb78860a6
action_a8047de7d06225c0e6db08be367fe651
action_b845e58f1426fb3f6362451bc6c71ae8
action_0ef0b9ae027bc192714520136c5a61ca
```

Update the context and affected documents with durable production evidence and
actual test IDs. Mark Phase 4D1 complete only when its acceptance criteria pass.
Push validated MiniAI and documentation commits, verify remote agreement, and
report results. Stop before Phase 4D2.
