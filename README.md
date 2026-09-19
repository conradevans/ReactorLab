# ReactorLab

ReactorLab is the infrastructure overview and monitoring control center for the ReactorLab Dell server.

MiniDeploy manages applications. MiniBase manages databases. ReactorLab observes the complete system and will later provide the data layer for MiniAI.

## Historical observability

The private Admin control plane records seven days of host, temperature,
per-application, and service metrics in a dedicated SQLite store. See
[Historical Observability v1](docs/historical-observability.md) for collection
intervals, schema behavior, bounded APIs, retention, security, and event-source
limitations.

## Guest resource aggregation

ReactorLab's public Guest View is a read-only showcase of resources that the
owning services have already chosen to share. ReactorLab does not own a second
visibility setting:

- MiniDeploy controls deployment visibility and ReactorLab reads only
  `GET /api/guest/deployments`.
- MiniBase controls database visibility and ReactorLab reads only
  `GET /api/v1/guest/databases`.
- ReactorLab never uses either service's Admin or private observability response
  to construct Guest data.

The browser uses the same-origin aggregate endpoint
`GET /api/v1/guest/resources`:

```json
{
  "deployments": {
    "available": true,
    "summary": {
      "total": 6,
      "shared": 4,
      "hidden": 2
    },
    "items": [
      {
        "app": "example",
        "url": "https://example.reactorlab.dev",
        "status": "running"
      }
    ]
  },
  "databases": {
    "available": true,
    "summary": {
      "total": 3,
      "shared": 2,
      "hidden": 1
    },
    "items": [
      {
        "id": "database_example",
        "displayName": "Example Database",
        "status": "ready"
      }
    ]
  }
}
```

ReactorLab converts each upstream `showing` count to `shared`, validates that
`shared` equals the number of returned items and that
`hidden == total - shared`, and constructs its own allowlisted DTO. Unknown
upstream fields are discarded. Hidden names, URLs, IDs, statuses, and other
details never enter the aggregation path.

The two upstream calls use the existing loopback service clients and bounded
five-second HTTP timeouts. They fail independently: an unavailable or malformed
MiniDeploy response marks only `deployments.available` false, and an
unavailable or malformed MiniBase response marks only
`databases.available` false. ReactorLab does not return raw upstream errors.

Deployment cards preserve the guest-safe public application URL returned by
MiniDeploy. Database cards are informational and provide no connection or Admin
link. Aggregate Total / Shared / Hidden counts are shown for each resource
type.

Detailed host telemetry—including CPU, memory, disk, temperature, battery,
load, uptime, network, process, container, and historical metrics—remains on the
private Admin surface. MiniAI diagnostics are not part of the public Guest
contract. The existing `GET /api/v1/guest/status` endpoint remains available
for compatibility, but the Guest resource page does not display its exact
uptime value.
