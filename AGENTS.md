# AGENTS.md — webtyp/ddl

<!-- webtyp:base — copied from devskills skill agents-md; do not edit here -->
## Ecosystem rules — read before any change

`docs/PLAN.md` is the spec: no design judgment; anything unclear is a question in the PR, not a guess.

- Public API: typed (no `any`/`interface{}`), explicit, minimal; never export a symbol only a test uses.
- One intent, one path: the old path is deleted in the same change. No silent fallback, `TODO`, commented code or stub.
- Fail at compile time, else a loud diagnostic — never silently. Zero values deny.
- A defect in a dependency is reported, never wrapped or patched here.
- No string literals in logic: named constants. The same logic never lives in two repos: import the package the plan names; if you find an existing one the plan does not name that already does what you are about to write, say so in the PR instead of writing it.
- `cmd/*/main.go`: parse flags, inject, print/exit only. stdout = data, stderr = logs, exit codes are the contract.
- WASM code: `webtyp/fmt`, `webtyp/time`, `webtyp/json` instead of stdlib; no `map`; `dom.Element` embedded by value.
- `css.go`, `svg.go`, `js.go`, `html.go` carry `//go:build !wasm`; no `ssr.go`, no `front.go`.
- Build tags: `wasm`/`!wasm` only when the API does not exist on that platform; `browser`/`!browser` when an app's file belongs to another binary; a library carries no destination tag (the linker drops what a binary never calls). One main per folder, no destination tag: `web/browser/main.go` → `web/public/browser.wasm`, `web/server/main.go` → `web/server/server`, `web/edge/main.go` → `web/edge/edge.wasm`.
- Code and keys in English; UI text is `lang.Text`, translations live in `lang.json`. Comments and docs: English in `webtyp/*`; in `veltylabs/*` **every README, `docs/*` file, diagram and code comment is in Spanish**, written for a reader with little technical background, and each exported symbol's comment shows a short usage example.
- A translation key is reused, never duplicated as a near-identical phrase (`"not a valid integer"` + `"not a valid decimal number"` repeats `"not a valid"` three ways instead of composing it once) — every library ships to WASM/TinyGo, so each unreused phrase is dictionary rows AND binary bytes, in every consumer, forever. A token that doesn't need translating (a kind/type name like `int`) is passed as data (a variable), never as a string literal — a literal becomes a dictionary key whether or not it needed one.
- Tests cover behaviour and domain rules, never the look (CSS, colours, markup order, class strings). In `tests/` (external package), real stack with fakes only at the edge; run `gotest`.
- Docs (`README.md`, `docs/*`) updated with the API; permanent docs never link `docs/PLAN.md`.
- Before the PR: `git diff --stat main` lists every file of the plan's stages, and nothing the plan did not order is deleted.
<!-- /webtyp:base -->

## This repo

Working notes for AI agents operating in this library. For end-user docs see [README.md](README.md) and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Some of this repo's algorithm used to live in
`webtyp/orm`, before the storage contract was extracted to `webtyp/storage` (the neutral DML port).

## Mission of this package

`webtyp/ddl` is the **runtime DDL** counterpart of
[`webtyp/storage`](https://github.com/webtyp/storage) (the storage port — contract + DML value
types + conformance + mem + mock). [`webtyp/orm`](https://github.com/webtyp/orm) is a **sibling**,
not a dependency: both `ddl` and `orm` sit on top of `storage`, but neither imports the other. `ddl`
owns schema management: `DropTable`/`CreateDatabase`/`Sync`/`SyncSchema`, plus
`ddl/conformance` — the executable contract SQL backends (`sqlt`, `postgres`) prove themselves against,
mirroring `storage/conformance` for DML. There is no public CreateTable: Sync creates a missing table and adds missing columns; a create-only call would leave installed databases without new fields. `webtyp/ddlc` stays the build-time codegen/CLI leaf
(`Exporter.ExportDDL`, `TopologicalSort`) — `ddl` is what *executes* the DDL that `ddlc` renders, at
runtime; `ddl` does not import `ddlc` as a library.

`ddl` depends on `storage` (`storage.Conn` for both `Exec` and, for `Sync`'s safe-drop probe, `Compile`
— see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)). **`ddl` never imports `orm`.** The dependency is
one-directional from `storage` outward: `storage` never imports `ddl` (or `orm`, or any backend).

## Architectural rules (do not violate)

### No Go `map` anywhere in this ecosystem

**Never use a built-in `map[K]V`, in any file, wasm-gated or not.** TinyGo's map runtime is heavy and
adds meaningful, unavoidable size to any wasm binary that ends up importing this code. This repo is
runtime code an executor adapter (`sqlt`, `postgres`) links into — including builds that end up in a
wasm target — so there is no "backend-only" exemption.

- For a **string→string** pair, use `webtyp.com/fmt.KeyValue{Key, Value string}`.
- For anything else (typed values, a table's columns, a rename map, a PK set), use a small local
  slice-of-structs and scan it linearly. Every collection this package touches is small (a model's
  field list, one table's columns), so a linear scan costs nothing in practice. See
  `webtyp/storage/mem`'s row/table pattern for the equivalent used elsewhere in the ecosystem.
- The `Sync` algorithm (`sync.go`) is map-free (`contains`/`schemaHasColumn`/`isRenameSource` helpers
  replace what would otherwise be `existingMap`/`schemaMap`/`renamedFrom`). The one exception:
  `RenameProvider.OldNames() map[string]string` is a **pre-existing external contract**
  (`ormc`-generated models across the ecosystem implement it) — this package only ever *reads* that
  caller-supplied map, it never allocates one of its own, so it doesn't violate the rule. Don't "fix"
  that signature here; it's out of scope and would ripple into `ormc` and every model that uses
  `db:"old_name=X"`.

### Root package (`ddl`) — orchestrates, never renders SQL

- **No direct SQL string building in `ddl`.** `ddl` decides *what* schema operation to run (`Stmt`/
  `Op`); the dialect's `Compiler.CompileDDL` decides *how* to render it as SQL. Don't hand-roll SQL
  strings here, even for something that looks trivial (e.g. `CREATE TABLE`).
- **No `database/sql` import.** Only `webtyp.com/storage` (for `Conn`/`Executor`/`Compiler`/
  `Query`/`Condition`/`TxExecutor`), `webtyp.com/model`, and `webtyp.com/fmt`.
  **Never `webtyp.com/orm`** — `ddl` and `orm` are siblings over `storage`, not a dependency
  of each other.
- **`ddl.DB` holds an `Execer` + a `ddl.Compiler`, nothing else.** `Execer` requires only `Exec`.
  `storage.Compiler`, `Query`, `storage.TxExecutor`, `TableIntrospector`, and `SchemaInspector`
  are all optional capabilities discovered by type assertion. `Sync`'s safe-drop probe uses
  `conn.Compile` / `conn.Query` when present; when absent, it takes an additive path or skips
  safe drops — no separate DML compiler argument is required. `ddl.DB` implements its own
  transaction wrapping (`boundConn`, `sync.go`) — it isn't an `orm.DB` and doesn't call anything
  from `orm`.
- **Do not use `tinygo` as a build tag** — not a real Go build constraint. Use `GOOS=js GOARCH=wasm` to
  build for wasm, `gotest -tinygo` to test against the TinyGo compiler specifically.

## Code layout

| File / Dir | Role |
|------------|------|
| `compiler.go` | `Op`, `Stmt` (incl. `ColumnName` for `OpDropColumn`), `Compiler` (DDL) — dialect adapters implement `CompileDDL` |
| `db.go` | `ddl.DB`, `New(conn storage.Conn, ddlCompiler Compiler)`, `DropTable`/`CreateDatabase` |
| `sync.go` | `Sync`/`SyncSchema` algorithm (map-free — see rule above), `boundConn` for transactions |
| `schema.go` | `TableIntrospector`, `SchemaInspector`, `ColumnInfo` |
| `conformance/` | Executable DDL contract (`Run(t, Factory)`), SQL backends only |
| `docs/` | `ARCHITECTURE.md` (durable design), `PLAN.md` when a new change is being planned (ephemeral, deleted after `gopush`) |

## Testing

Install once:

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Run:

```bash
gotest              # vet + race + cover + wasm + badges
gotest -no-cache    # force re-run
gotest -run TestX   # filter
```

Publish with `gopush 'message'` (tests + tag + push) — never `git commit`/`git push` directly.

## Common mistakes to avoid

- Reaching for `map[K]V` for a lookup table, a rename map, or a PK set → use `fmt.KeyValue` or a small
  local slice-of-structs scanned linearly instead. No exceptions.
- Rendering SQL directly in `ddl` instead of going through the dialect's `Compiler.CompileDDL`.
- Importing `webtyp.com/orm` for anything → `ddl` depends on `storage`, never on `orm`. If you
  find yourself wanting `orm.X`, the type you need is `storage.X`.
- Adding a third constructor argument (a separate DML compiler) to `ddl.New` → `storage.Conn` already
  carries `Compile`, that's the DML compiler. Two arguments (`conn`, `ddlCompiler`) is correct.
