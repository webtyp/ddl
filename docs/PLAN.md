---
PLAN: "fix!: CreateTable is no longer public — Sync is the one way to create and evolve a table"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 12270730623465071777
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# `CreateTable` leaves the public API

## Context

`DB.CreateTable` compiles to `CREATE TABLE IF NOT EXISTS`: against an existing table it does nothing, so every module
that migrated with it never delivered a model's new field to an installed database — the first read or write of that
field failed in production. `DB.Sync` creates a missing table **and** adds the missing columns
(`docs/diagrams/SYNC_FLOW.md`). All 15 libraries and modules that called `CreateTable`, and every test outside this
repo, now call `Sync` (verified: `grep -rn "\.CreateTable(" --include=*.go` over `webtyp/` and `veltylabs/` finds only
this repo). This plan closes the hole for good: `CreateTable` becomes unexported, so writing it again does not compile.
`DropTable` and `CreateDatabase` stay public (they have no "evolve" counterpart).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`.

## Design gate

1. **Prior art.** Django and Rails expose migrations, not a bare "create if missing" that silently skips existing
   tables; GORM's `AutoMigrate` creates and adds columns in one call (unverified: pages not read in this session).
2. **Novice name.** `db.Sync(models...)` — the only call a consumer writes.
3. **Complexity ledger.** Ways to create a table −1 (from two to one).
4. **Where it belongs.** `ddl`.
5. **Deletes.** The exported `(*DB).CreateTable`; `TestCreateTable` (its case is covered by the existing Sync test whose
   comment says "Only CreateTable is called since columns match").

## Stage 1 — code and this repo's tests

- `db.go`: rename `func (d *DB) CreateTable(m model.Model) error` to `func (d *DB) createTable(m model.Model) error`;
  its comment becomes: `// createTable emits CREATE TABLE IF NOT EXISTS for m: Sync's first step. Not public: on its own it never evolves an existing table.`
- `sync.go:116`: call `d.createTable(m)`.
- `conformance/conformance.go`: every `schema.CreateTable(w)` becomes `schema.Sync(w)`; failure texts `"CreateTable failed"`
  → `"Sync failed"`, `"first CreateTable failed"` → `"first Sync failed"`, `"second CreateTable (idempotency) failed"`
  → `"second Sync (idempotency) failed"`. Subtest names stay as they are.
- `capture_test.go:15`: `ddl.New(conn, c).CreateTable(&mock.Model{Table: "tenant"})` → `ddl.New(conn, c).Sync(&mock.Model{Table: "tenant"})`
  (Capture records only CREATE TABLE statements, so the expected tables do not change).
- `ddl_test.go`: delete `TestCreateTable` (lines 37-58). Comments that mention "CreateTable" as the emitted statement
  stay (they describe `OpCreateTable`).

Run `gotest`: green. `grep -rn "\.CreateTable(\|func (d \*DB) CreateTable" --include=*.go .` → empty. Commit.

## Stage 2 — docs

- `README.md:5`, `docs/ARCHITECTURE.md:7`, `AGENTS.md:36` and `AGENTS.md:92`: the public list becomes
  `DropTable`/`CreateDatabase`/`Sync`/`SyncSchema`; add one sentence where the list is: «There is no public
  CreateTable: Sync creates a missing table and adds missing columns; a create-only call would leave installed databases
  without new fields.»
- `docs/ARCHITECTURE.md:58` («1. `CreateTable` per model (idempotent).») → «1. `CREATE TABLE IF NOT EXISTS` per model (the private `createTable`).»

Commit.

## Delivery check

1. `gotest` passes. A PR that does not compile, or whose tests could not run, is a failed delivery.
2. `grep -rn "\.CreateTable(\|func (d \*DB) CreateTable" --include=*.go .` → empty.
3. `git diff --stat main` lists only `db.go`, `sync.go`, `conformance/conformance.go`, `capture_test.go`, `ddl_test.go`,
   `README.md`, `docs/ARCHITECTURE.md`, `AGENTS.md`.

## Delivery rules

- Commit after every stage. Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
- Stuck for 30 minutes: open the PR anyway, listing what fails.

## Stages

| # | Files | Result |
|---|---|---|
| 1 | `db.go`, `sync.go`, `conformance/conformance.go`, `capture_test.go`, `ddl_test.go` | `CreateTable` private; green |
| 2 | `README.md`, `docs/ARCHITECTURE.md`, `AGENTS.md` | Docs say Sync is the one way |
