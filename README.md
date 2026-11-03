# ddl
<img src="docs/img/badges.svg">

Runtime DDL over [webtyp/storage](https://github.com/webtyp/storage):
DropTable/CreateDatabase/Sync/SyncSchema, plus the executable `ddl/conformance` contract for
SQL backends. There is no public CreateTable: Sync creates a missing table and adds missing columns; a create-only call would leave installed databases without new fields. A sibling of webtyp/orm, not a dependency of it — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the design. How `Sync` migrates, branch by branch: [docs/diagrams/SYNC_FLOW.md](docs/diagrams/SYNC_FLOW.md).

## Capture — the tables a migration list creates

`ddl.Step{Name, Run}` names one module's migration (`Run` has the signature of every module's `Migrate`).
`ddl.Capture(steps)` runs them against a recorder that touches no database and returns every created table,
in order, with its step — what `webtyp.com/erd` draws as an app's database diagram.

## Backups — read any engine's backup the same way

`ddl.BackupFormat` is what an engine knows about its own backup files: `OpenBackup(path)` returns a read-only
`ddl.Snapshot` — its tables and columns (`SchemaInspector`) and each table's rows as text (`ReadTable`), with no dialect
SQL in the caller, and `Notices` — plain sentences about anything the engine tolerated while opening it. `Close` releases it. Implemented by each engine (`webtyp.com/postgres`: a `pg_dump` archive);
consumed by `webtyp.com/dataimport` to bring a legacy system's data into an app. Contract: [backup.go](backup.go).
