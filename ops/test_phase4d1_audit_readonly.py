import hashlib
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest

import phase4d1_audit_readonly as audit


class AuditReadOnlyTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="phase4d1-audit-test-")
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.backup = self.directory / "miniai-v3.db"
        self.live = self.directory / "live.db"
        self.baseline = {}
        connection = sqlite3.connect(self.backup)
        for table in audit.TABLES:
            connection.execute("CREATE TABLE " + table + " (id TEXT PRIMARY KEY, value TEXT)")
            identifiers = audit.FIXTURES if table == "action_proposals" else ("historical",)
            for identifier in identifiers:
                row = (identifier, "preserved")
                connection.execute("INSERT INTO " + table + " VALUES (?, ?)", row)
                self.baseline.setdefault(table, {})[identifier] = hashlib.sha256(
                    json.dumps(row, separators=(",", ":")).encode()
                ).hexdigest()
        connection.execute("PRAGMA user_version=3")
        connection.commit()
        live = sqlite3.connect(self.live)
        connection.backup(live)
        live.execute("PRAGMA user_version=4")
        live.execute("INSERT INTO action_proposals VALUES ('new', 'valid later activity')")
        live.commit()
        live.close()
        connection.close()
        (self.directory / "audit-before.json").write_text(json.dumps(self.baseline))

    def test_original_hash_format_and_new_activity_pass_without_file_changes(self):
        before = {path.name: path.read_bytes() for path in self.directory.iterdir()}
        result = audit.verify(self.live, self.directory)
        self.assertEqual(result["result"], "PASS")
        self.assertEqual(result["new_record_counts"]["action_proposals"], 1)
        self.assertEqual(before, {path.name: path.read_bytes() for path in self.directory.iterdir()})

    def test_changed_historical_row_fails(self):
        with sqlite3.connect(self.live) as connection:
            connection.execute("UPDATE action_events SET value='changed'")
        with self.assertRaisesRegex(ValueError, "Historical rows missing or changed"):
            audit.verify(self.live, self.directory)

    def test_missing_baseline_is_not_recreated(self):
        baseline = self.directory / "audit-before.json"
        baseline.unlink()
        with self.assertRaises(FileNotFoundError):
            audit.verify(self.live, self.directory)
        self.assertFalse(baseline.exists())

    def test_replacement_baseline_fails_against_original_backup(self):
        self.baseline["action_events"]["historical"] = "0" * 64
        (self.directory / "audit-before.json").write_text(json.dumps(self.baseline))
        with self.assertRaisesRegex(ValueError, "backup does not match"):
            audit.verify(self.live, self.directory)

    def test_missing_live_database_is_not_created(self):
        missing = self.directory / "missing.db"
        with self.assertRaises(sqlite3.OperationalError):
            audit.verify(missing, self.directory)
        self.assertFalse(missing.exists())


if __name__ == "__main__":
    unittest.main()
