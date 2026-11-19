"""Read-only Phase 4D1 verification against the preserved pre-deployment baseline.

Run with python3 -I -B. No baseline creation, migration, restore, or write mode.
The hashing format intentionally matches the original deployment audit helper.
"""

import hashlib
import json
from pathlib import Path
import sqlite3
import sys


TABLES = ("action_proposals", "action_events", "history_entries")
FIXTURES = (
    "action_6b29ccf5ff1eb4312a2c5b6769be8e90",
    "action_ec81dad4db3e2a1d536c326bb78860a6",
    "action_a8047de7d06225c0e6db08be367fe651",
    "action_b845e58f1426fb3f6362451bc6c71ae8",
    "action_0ef0b9ae027bc192714520136c5a61ca",
)
BASELINE_DIRECTORY = Path("/var/lib/miniai/phase4d1-87cd296")
LIVE_DATABASE = Path("/var/lib/miniai/miniai.db")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def snapshot(database):
    # Do not use immutable=1: the live database may have committed WAL data.
    connection = sqlite3.connect(database.resolve().as_uri() + "?mode=ro", uri=True)
    try:
        connection.execute("PRAGMA query_only=ON")
        connection.execute("BEGIN")
        require(connection.execute("PRAGMA integrity_check").fetchall() == [("ok",)],
                "SQLite integrity check failed")
        require(not connection.execute("PRAGMA foreign_key_check").fetchall(),
                "SQLite foreign-key check failed")
        version = connection.execute("PRAGMA user_version").fetchone()[0]
        rows = {}
        for table in TABLES:
            rows[table] = {
                row[0]: hashlib.sha256(
                    json.dumps(row, separators=(",", ":")).encode()
                ).hexdigest()
                for row in connection.execute("SELECT * FROM " + table)
            }
        return version, rows
    finally:
        connection.close()


def verify(database=LIVE_DATABASE, baseline_directory=BASELINE_DIRECTORY):
    # Both must already exist. Never replace a missing baseline with current data.
    baseline_bytes = (baseline_directory / "audit-before.json").read_bytes()
    baseline = json.loads(baseline_bytes)
    require(isinstance(baseline, dict) and set(baseline) == set(TABLES),
            "Unexpected pre-deployment baseline structure")
    require(all(isinstance(baseline[table], dict) for table in TABLES),
            "Unexpected pre-deployment baseline rows")

    backup_version, backup = snapshot(baseline_directory / "miniai-v3.db")
    require(backup_version == 3, "Original backup is not schema v3")
    require(backup == baseline, "Original v3 backup does not match saved baseline hashes")
    require(all(identifier in baseline["action_proposals"] for identifier in FIXTURES),
            "Original baseline is missing a required historical fixture")

    version, current = snapshot(database)
    require(version == 4, "Live database is not schema v4")
    for table in TABLES:
        require(all(current[table].get(identifier) == digest
                    for identifier, digest in baseline[table].items()),
                "Historical rows missing or changed in " + table)
    return {
        "result": "PASS",
        "schema": version,
        "baseline_schema": backup_version,
        "baseline_sha256": hashlib.sha256(baseline_bytes).hexdigest(),
        "baseline_counts": {table: len(baseline[table]) for table in TABLES},
        "current_counts": {table: len(current[table]) for table in TABLES},
        "new_record_counts": {
            table: len(current[table]) - len(baseline[table]) for table in TABLES
        },
        "historical_hashes_preserved": True,
        "fixtures_preserved": len(FIXTURES),
        "integrity": "ok",
        "foreign_key_violations": 0,
    }


if __name__ == "__main__":
    try:
        require(len(sys.argv) == 1, "This helper accepts no arguments or write modes")
        print(json.dumps(verify(), sort_keys=True))
    except (OSError, ValueError, sqlite3.Error) as error:
        print("FAIL: " + str(error), file=sys.stderr)
        sys.exit(1)
