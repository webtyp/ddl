# Last plan executed locally — ddl.Snapshot and ddl.BackupFormat

Contract that unblocks `webtyp/postgres` (implementation) and `webtyp/dataimport` (consumer).

## Ecosystem search
- `ddl.SchemaInspector` (Tables/Columns) and `storage.Rows` already exist → **reused**.
- No repo opens a backup file as a readable database; `selfupdate/compose.go` runs `pg_dump` inside the
  Docker container (writes a backup, never reads one) → not the same concern, left as is.
- **New**: `backup.go` in `ddl`, the package that already holds the engine-administration contracts.

## Design gate
1. Prior art: `pg_restore` (restore archive into a database), SQLite backup API (a backup is a database file),
   Django `dumpdata`/`loaddata` (engine-neutral read) — unverified links. Shared standard: restore, then read with
   the engine's own reader; followed.
2. Names: `OpenBackup`, `Snapshot`, `ReadTable` read as plain sentences.
3. Ledger: concepts +2 (Snapshot, BackupFormat); ways to read a backup: 0 → 1.
4. Belongs in `ddl` (contracts); implementations in each engine repo (O: a new engine touches zero consumers).
5. Deletes nothing: new capability.

## Done
- `backup.go`, `backup_test.go`, README section. `gotest` green.
