# Architecture — webtyp/ddl

## What this package is

`webtyp/ddl` is the **runtime DDL** counterpart of [`webtyp/storage`](https://github.com/webtyp/storage)
(the neutral DML port: contract + value types + conformance + mem + mock). It owns schema management —
`DropTable`/`CreateDatabase`/`Sync`/`SyncSchema` — and the executable contract
`ddl/conformance` that SQL backends (`sqlt`, `postgres`) prove themselves against, mirroring
`storage/conformance` for DML. There is no public CreateTable: Sync creates a missing table and adds missing columns; a create-only call would leave installed databases without new fields.

[`webtyp/orm`](https://github.com/webtyp/orm) is a **sibling**, not a dependency: both `ddl` and
`orm` sit on top of `storage`, but neither imports the other. `webtyp/ddlc` is a separate, unrelated
build-time codegen/CLI leaf (`Exporter.ExportDDL`, `TopologicalSort`) that *renders* SQL DDL; `ddl`
*executes* schema changes at runtime through a `storage.Conn` — it does not import `ddlc`.

Dependency direction is one-way: `storage` → `ddl`. `storage` never imports `ddl` or `orm`.

## Why the split from `orm`

`webtyp/orm` used to mix DML (operating on data) and DDL (creating/migrating schema) on the same
`*orm.DB`. That coupled every backend adapter to the full ORM surface just to satisfy interfaces. The
DML contract was extracted to the neutral port `webtyp/storage`; DDL was extracted here. `ddl` depends
only on `storage`, never on `orm` — so backends that need schema management don't have to pull in an
ORM to get it.

## Core types

- **`ddl.Op`** — a DDL operation (`OpCreateTable`, `OpDropTable`, `OpCreateDatabase`, `OpAddColumn`,
  `OpRenameColumn`, `OpDropColumn`, `OpCopyColumn`), distinct from `storage`'s DML `Action`.
- **`ddl.Stmt`** — one DDL statement to compile: `Op`, `Table`, `Database`, `Column *model.Field` (full
  field metadata for `AddColumn`/`RenameColumn`, which need type info), `OldName` (for `RenameColumn`, and the
  source of `CopyColumn`), and `ColumnName string` (bare name, for `OpDropColumn` and the destination of `OpCopyColumn` — dropping needs no type, so it doesn't
  force callers to fabricate a fake `model.Field`).
- **`ddl.Compiler`** — implemented by dialect adapters (`sqlt`, `postgres`). Renders a `Stmt` to engine
  SQL: `CompileDDL(s Stmt, m model.Model) (query string, args []any, err error)`. The DDL counterpart of
  `storage.Compiler`, which stays DML-only. `ddl` orchestrates *what* to run; the dialect decides *how*
  to render it — no SQL string building in this package.
- **`ddl.DB`** — the runtime. Holds a `storage.Conn` and a `ddl.Compiler`, nothing else.
  `storage.Conn` already unifies Executor+Compiler(DML), so `Sync`'s safe-drop probe (a real DML SELECT)
  uses `conn.Compile` directly — no separate DML compiler argument.
  `New(conn storage.Conn, ddlCompiler Compiler) *DB` — two arguments.

## Schema introspection (optional capabilities)

A `storage.Conn` implementation may optionally implement:

- **`TableIntrospector`** — `TableColumns(table string) ([]string, error)`. Used by `Sync` to diff the
  model's field list against what's actually in the table. Without it, `Sync` falls back to a purely
  additive loop (best-effort `AddColumn` for every field, errors logged and skipped).
- **`SchemaInspector`** — `Tables() ([]string, error)` and `Columns(table string) ([]ColumnInfo, error)`.
  Broader introspection, consumed outside this package (e.g. an MCP `db_schema` tool); if the adapter
  doesn't implement it, that consumer simply doesn't register.

## The `Sync` algorithm

`Sync(models ...model.Model)` reconciles the database schema to match the given models:

1. `CREATE TABLE IF NOT EXISTS` per model (the private `createTable`).
2. If `conn` doesn't implement `TableIntrospector`: additive-only `AddColumn` per field, best-effort.
3. Otherwise, read `existingCols` via `TableColumns`.
4. If the model implements `RenameProvider` (`OldNames() map[string]string` — a pre-existing external
   contract that `ormc`-generated models satisfy when a field has a `db:"old_name=X"` tag), use it to
   distinguish a genuine rename from a plain add.
5. For each schema field missing from `existingCols`: emit `OpRenameColumn` if its old name is present,
   else `OpAddColumn`.
6. For each existing column no longer in the schema (and not the source of a rename): a **safe-drop**
   check — `SELECT 1 FROM <table> WHERE <col> IS NOT NULL LIMIT 1` compiled and run through `conn`'s DML
   half. Only emits `OpDropColumn` if the column has no data; otherwise skips and logs (listed by `Leftovers`).

`Sync` runs inside a transaction when `conn` implements `storage.TxExecutor`: it composes a `boundConn`
that re-pairs the transaction-bound `Executor` with the original connection's `Compiler` (and, when
present, its `TableIntrospector`/`SchemaInspector`) — compiling doesn't depend on being inside a
transaction, only executing does. On any error the transaction rolls back; otherwise it commits. Without
a `TxExecutor`, `Sync` runs the same reconciliation directly against `conn`, uncommitted.

`SyncSchema(table string, fields []model.Field)` is `Sync` for callers that have a raw field list
instead of a `model.Model` — it wraps them in an internal model shim.

## Leftover columns

`Sync` keeps a column the model no longer has when it still holds data. `Leftovers(tables, removed)` lists those
columns, read-only: for each table from `Capture` (read once even if listed twice) it counts the rows with a value up
to `LeftoverRowCap` (`More` beyond it — no table is read whole), keeps `LeftoverSamples` example values and attaches the
app's `Removed` declaration for that column (where the data went). The old name of a renamed field is never listed.
`DropColumn(table, column)` and `CopyColumn(removed)` (`OpCopyColumn`: fills the destination only where it is NULL)
act on one column, only by a person's decision — `webtyp dev` asks, production never drops.
An app hands both halves around as one value, `Schema{Steps, Removed}`: the steps are regenerated from the app's
modules on every build, while `Removed` holds the developer's answers and is never regenerated, so they live in two
files but travel together.
Flow and branches: [diagrams/LEFTOVERS_FLOW.md](diagrams/LEFTOVERS_FLOW.md).

## Constraints

- **No `map[K]V`, anywhere.** TinyGo's map runtime is heavy and adds unavoidable size to any wasm binary
  that imports this code — this package is runtime code that backend adapters link into, with no
  "backend-only" exemption. Every collection here is small (a model's field list, one table's columns),
  so linear scans (`contains`/`schemaHasColumn`/`isRenameSource`) cost nothing in practice. The one
  exception is `RenameProvider.OldNames()`, a pre-existing external `map[string]string` contract that
  this package only ever reads, never allocates.
- **No SQL string building in `ddl`.** All rendering goes through `Compiler.CompileDDL`.
- **No `database/sql` import.** Only `storage`, `model`, `fmt`.
- **Never imports `orm`.**

## `ddl/conformance`

`package conformance`, mirroring `storage/conformance`. A `Factory` supplies a fresh `ddl.DB` plus the
underlying `storage.Conn` and a column-introspection closure; `Run(t, Factory)` exercises table
creation, idempotency, `Sync`'s add-column path, table drop, `CopyColumn` (only empty rows) and `DropColumn` of a
column with data, using `conformance.Widget` from
`webtyp.com/storage/conformance` (not duplicated here). Only SQL backends (`sqlt`, `postgres`)
run it — `indexdb`/`storage/mem` don't do DDL.
