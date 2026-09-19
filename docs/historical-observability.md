# Historical Observability v1

ReactorLab owns historical infrastructure telemetry in a dedicated SQLite
database. This is separate from the existing `reactorlab.db` monitoring
activity store and does not depend on MiniBase or PostgreSQL.

## Runtime and storage

The default database is `/srv/reactorlab/data/observability.db`. Override it
with `-observability`. MiniDeploy, MiniBase, and MiniAI management URLs are
configurable with `-minideploy-url`, `-minibase-url`, and `-miniai-url`.

The database directory is restricted to `0700` and the main database file to
`0600`. SQLite uses WAL mode, a five-second busy timeout, foreign keys, schema
migrations, a persistent handle, and a single bounded writer pipeline. Related
samples are written in short transactions. The collector queue is bounded;
dropping an occasional metric under sustained writer pressure is preferable to
unbounded memory growth. No full `VACUUM` runs on the sampling path.

If this database cannot be initialized, ReactorLab continues serving its
existing Admin and Guest functionality. The private Observability API returns
`503 observability_unavailable` instead of pretending history exists.

## Collection

- Host CPU, numeric memory, load 1/5/15, root-filesystem capacity, disk I/O,
  and host network throughput are sampled every five seconds.
- CPU uses `/proc/stat` deltas. The first observation is stored without a
  fabricated percentage.
- Disk I/O discovers the block device backing `/` from mount information and
  records one parent device, avoiding partition/parent double counting. If `/`
  has no discoverable block device, disk capacity still records while I/O is
  unavailable.
- Host networking uses only the non-loopback interface selected by the Linux
  default route. This avoids summing Docker bridges and veth pairs twice.
- Temperature reuses the live `k10temp`/thermal-zone sysfs semantics. It polls
  every second and persists five-second buckets containing min, weighted
  average, max, peak timestamp, and sample count. A partial bucket is flushed
  during shutdown. Missing sensors do not stop any other collector.
- MiniDeploy's existing batched ReactorLab observability endpoint supplies
  per-application CPU, memory, cumulative network counters, status, and restart
  count every ten seconds. Canonical MiniDeploy app names are stable IDs.
  Network rates reset to unavailable on a first observation, counter reset, or
  container-set replacement.
- ReactorLab, MiniDeploy, MiniBase, and MiniAI health is sampled every ten
  seconds with independent bounded local probes. ReactorLab self-health is
  internal rather than recursive HTTP. When readable without elevation,
  systemd invocation IDs identify service restarts.

All counter rates use actual elapsed time. Counter decrease, replacement, or
invalid elapsed time produces an unavailable rate rather than a negative or
artificial spike.

The collector starts under ReactorLab's process context and stops before the
observability store closes. Each cadence is serialized, all tickers stop on
shutdown, temperature attempts a final partial flush, and repeated failures
are log-rate-limited.

## Retention and queries

Host, temperature, application, and service metric rows are retained for
exactly seven days and pruned hourly in an indexed transaction. Events, alert
rules, alert incidents, and collector state are not pruned by metric retention.

The typed Go query service is the only historical read boundary. It supports
the fixed `15m`, `1h`, `6h`, `24h`, and `7d` windows and caps display series at
720 points. Five-second resolution is used for 15 minutes and one hour,
one-minute buckets for six hours, three-minute buckets for 24 hours, and
15-minute buckets for seven days. Rate metrics use averages and maxima, not
sums. Temperature uses sample-count-weighted averages while retaining the
highest maximum and its original peak timestamp.

This bounded service is ready for a later MiniAI integration. MiniAI does not
query SQLite and is not integrated in v1.

## Private Admin API and UI

The following routes exist only on ReactorLab's private Admin listener:

- `GET /api/v1/observability/host?range=1h`
- `GET /api/v1/observability/temperature?range=1h`
- `GET /api/v1/observability/apps?range=1h`
- `GET /api/v1/observability/apps/{id}?range=1h`
- `GET /api/v1/observability/services?range=1h`
- `GET /api/v1/observability/events?range=1h`

Responses include the accepted range, UTC bounds, and an allowlisted typed data
payload. The public Guest listener does not register any Observability path.

`/admin/observability` provides the five range controls, native SVG host and
per-application charts, separate average/peak temperature series, compact
service availability history, and the infrastructure event timeline. It
refreshes periodically rather than polling at collector frequency.

## Events and current source limits

Generated events cover health outage/recovery transitions, application restart
count changes, systemd-detectable platform service restarts, and Linux boot-ID
changes. Initial service and boot observations establish baselines and do not
fabricate outage, recovery, or reboot events.

MiniBase's management activity API is authoritative for database create,
delete, manual/automatic backup, and restore events. Because MiniBase activity
records do not expose IDs, ReactorLab derives a deterministic ID from immutable
safe fields and the exact source timestamp. Inserts are idempotent by
`source + source_event_id`.

Current upstream limitations are explicit:

- MiniBase does not record Guest visibility changes in its activity feed.
- MiniDeploy version history does not identify initial deploy versus redeploy
  versus rollback, and its timestamps describe history movement rather than a
  stable operation event. ReactorLab therefore does not guess those event
  distinctions in v1.
- Exact service restart events require a readable systemd invocation ID. Health
  transitions continue when it is unavailable.

No credentials, environment variables, connection strings, tokens, arbitrary
logs, or raw Docker inspection objects are persisted.

## Alert-ready persistence

Schema v1 includes alert rule and incident repositories. Rules contain scope,
optional resource ID, metric, operator, numeric threshold, duration seconds,
enabled state, and created/updated timestamps. Incidents contain the rule,
resource, open/last-observed/resolved times, and state. Millisecond sample
timestamps and explicit durations support future sustained-condition
evaluation. V1 adds no rule editor, notifications, webhooks, email, or other
delivery system.
