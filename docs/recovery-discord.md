# Recovery Discord delivery

ReactorLab can create one durable Discord webhook outbox item for each newly
detected live unexpected-shutdown recovery incident. The feature defaults to
disabled and is enabled only when `REACTORLAB_RECOVERY_DISCORD_ENABLED=true`.
Historical incident writes use `Notify: false` and do not create webhook work.

The service reads non-secret settings from the optional
`/etc/reactorlab/recovery-discord.env` file. Use
`deploy/recovery-discord.env.example` as the template. The optional
`REACTORLAB_DISCORD_USERNAME` changes the webhook display name; when omitted,
Discord's configured default identity is used.

The webhook URL is read only when delivery is attempted from the absolute path
in `REACTORLAB_DISCORD_WEBHOOK_FILE`. It is not stored in an environment
variable, SQLite, a log, an API response, or a frontend asset. A compatible
installation is:

```text
/etc/reactorlab                       root:reactorlab 0750
/etc/reactorlab/recovery-discord.env  root:root       0600
/etc/reactorlab/discord-webhook       root:reactorlab 0640
```

The webhook secret must be in a regular, non-symlink file. It must not be
world-accessible, group-writable, executable, or unreadable. A trailing newline
is accepted. ReactorLab requires an HTTPS URL with a Discord webhook-shaped
`/api/webhooks/...` path and does not include the rejected value in errors.
ReactorLab does not create or change this file or its permissions.

`REACTORLAB_RECOVERY_NOTIFICATION_TIMEZONE` controls human-readable timestamps
in messages. Use an IANA timezone such as `America/New_York` for automatic
EDT/EST handling. The default is UTC. An invalid timezone degrades safely to UTC
and does not prevent ReactorLab from starting.

For a later authorized deployment:

1. Install the example as `/etc/reactorlab/recovery-discord.env`.
2. Put the real Discord webhook URL directly in
   `/etc/reactorlab/discord-webhook`; do not add it to this repository.
3. Apply the ownership and modes above.
4. Set `REACTORLAB_RECOVERY_DISCORD_ENABLED=true`.
5. Deploy and restart ReactorLab through the normal authorized process.

Enablement and delivery readiness are separate. When disabled, recovery
detection continues but no new outbox row or worker is created. When enabled,
a new live incident and its outbox row are committed atomically even if the
secret file is missing, unreadable, or invalid. The independent worker starts
without blocking ReactorLab, records a bounded error category, and retries
without a hot loop.

The persisted retry schedule is one minute, five minutes, fifteen minutes,
thirty minutes, one hour, then hourly. A valid Discord `Retry-After` value can
extend the next attempt. Network failures, rate limits, server failures, and
configuration-style failures remain retryable so an operator can repair the
webhook later. Provider response bodies and webhook URLs are never persisted.

Immediately before each attempt, ReactorLab reads the current aggregate,
hardware-watchdog, and RTC recovery-protection state. The message reports these
as current post-restart state and does not claim that any mechanism caused the
recovery.

The outbox guarantees one durable logical notification row per recovery event.
The HTTPS request and SQLite sent-state update cannot be atomic: if Discord
accepts a POST and ReactorLab exits before recording `sent`, a later retry can
produce a duplicate message. Successfully recorded sent rows are not retried.
