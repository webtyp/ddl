# ddl
<img src="docs/img/badges.svg">

Runtime DDL over [webtyp/storage](https://github.com/webtyp/storage):
CreateTable/DropTable/CreateDatabase/Sync/SyncSchema, plus the executable `ddl/conformance` contract for
SQL backends. A sibling of webtyp/orm, not a dependency of it — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the design. How `Sync` migrates, branch by branch: [docs/diagrams/SYNC_FLOW.md](docs/diagrams/SYNC_FLOW.md).

## Capture — the tables a migration list creates

`ddl.Step{Name, Run}` names one module's migration (`Run` has the signature of every module's `Migrate`).
`ddl.Capture(steps)` runs them against a recorder that touches no database and returns every created table,
in order, with its step — what `webtyp.com/erd` draws as an app's database diagram.
