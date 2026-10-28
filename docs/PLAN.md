---
PLAN: "feat: Step and Capture — the tables a list of migrations creates, without a database"
TAG: v0.0.32
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 8581247014584470465
PR: https://github.com/webtyp/ddl/pull/3
REVIEWED_COMMIT: 635ee383aeb5d3e6771ee4599c25b8010253ca72
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/ddl`: `Step` and `Capture`

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Why

An app migrates its database as an ordered list of module migrations (`mjosefa-cms/config/migrate.go` declares
its own `MigrationStep{Name, Run}` for that). The app's database diagram (`webtyp.com/erd`) needs every table
of that list, grouped by module, without opening a database: every module's `Migrate(conn, compiler)` already
ends in `ddl.DB.CreateTable`/`Sync`, which hand each model to the `Compiler`. A recording `Execer`+`Compiler`
collects them.

## Ecosystem search

`ddl.Execer`, `ddl.Compiler`, `ddl.Stmt{Op: OpCreateTable}` exist here. The step type exists only inside
`mjosefa-cms/config` (an app). Extend `ddl`: the step is the missing contract between apps, `appkit` and `erd`.

## Design gate

1. **Prior art:** goose and golang-migrate (an ordered list of named migrations), Django `sqlmigrate` / Rails
   `db:schema:dump` (the schema a migration list produces, computed without applying it). `Capture` is the
   dry-run of that list.
2. **Names:** `ddl.Step{Name, Run}` — «a migration step»; `ddl.Capture(steps)` — «capture what the steps create»;
   `ddl.Table{Step, Model}`.
3. **Ledger:** concepts +3 (`Step`, `Table`, `Capture`) / −1 (each app's own step struct); ways to list an app's
   tables +1 / −0 (there was none).
4. **Belongs:** `ddl` owns `Execer`, `Compiler` and the statements; the recorder is one more implementation of
   them. No SQL is rendered (AGENTS.md: `ddl` never builds SQL).
5. **Deletes:** `mjosefa-cms`'s `config.MigrationStep` (in that app's own plan, later).

## Stage 1 — `capture.go` (new)

No `map` (AGENTS.md): slices only.

```go
package ddl

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
)

// Step is one module's migration, named so an error and the database diagram say which module it is.
//
//	ddl.Step{Name: "rbac", Run: rbac.Migrate}
type Step struct {
	Name string
	Run  func(conn Execer, compiler Compiler) error
}

// Table is a table a Step creates.
type Table struct {
	Step  string
	Model model.Model
}

const errCapture = "ddl: capture: step"

// Capture runs every step, in order, against a recorder that touches no database, and returns every table the
// steps create (each OpCreateTable), in order. Raw Exec calls and every other statement are accepted and
// ignored. A step that fails stops it: the error names the step.
//
//	tables, err := ddl.Capture(config.MigrationSteps())
func Capture(steps []Step) ([]Table, error) {
	rec := &recorder{}
	for _, s := range steps {
		rec.step = s.Name
		if err := s.Run(rec, rec); err != nil {
			return nil, fmt.Err(errCapture, s.Name+":", err.Error())
		}
	}
	return rec.tables, nil
}

// recorder is an Execer and a Compiler that only remembers which models a step creates.
type recorder struct {
	step   string
	tables []Table
}

func (r *recorder) Exec(string, ...any) error { return nil }

func (r *recorder) CompileDDL(s Stmt, m model.Model) (string, []any, error) {
	if s.Op == OpCreateTable {
		r.tables = append(r.tables, Table{Step: r.step, Model: m})
	}
	return "", nil, nil
}
```

`recorder` implements neither `storage.TxExecutor`, `storage.Compiler`, `TableIntrospector` nor
`SchemaInspector`, so `DB.Sync` takes its additive path: `CreateTable` (recorded) then one `OpAddColumn` per
field (ignored). Do not add those interfaces.

## Stage 2 — `capture_test.go` (new, package `ddl_test`, same style as `ddl_test.go`)

Use `webtyp.com/storage/mock` `&mock.Model{Table: "<name>"}` as the models.

```go
func TestCapture_ListsEveryCreatedTableInOrderWithItsStep(t *testing.T) {
	steps := []ddl.Step{
		{Name: "auth", Run: func(conn ddl.Execer, c ddl.Compiler) error {
			return ddl.New(conn, c).CreateTable(&mock.Model{Table: "tenant"})
		}},
		{Name: "rbac", Run: func(conn ddl.Execer, c ddl.Compiler) error {
			if err := ddl.New(conn, c).Sync(&mock.Model{Table: "role"}, &mock.Model{Table: "user_role"}); err != nil {
				return err
			}
			return conn.Exec("CREATE UNIQUE INDEX IF NOT EXISTS role_code ON role(code)")
		}},
	}
	tables, err := ddl.Capture(steps)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ step, table string }{{"auth", "tenant"}, {"rbac", "role"}, {"rbac", "user_role"}}
	if len(tables) != len(want) {
		t.Fatalf("got %d tables, want %d", len(tables), len(want))
	}
	for i, w := range want {
		if tables[i].Step != w.step || tables[i].Model.ModelName() != w.table {
			t.Errorf("tables[%d] = %s/%s, want %s/%s", i, tables[i].Step, tables[i].Model.ModelName(), w.step, w.table)
		}
	}
}

func TestCapture_AFailingStepIsNamed(t *testing.T) {
	steps := []ddl.Step{{Name: "billing", Run: func(ddl.Execer, ddl.Compiler) error { return errors.New("boom") }}}
	_, err := ddl.Capture(steps)
	if err == nil || !strings.Contains(err.Error(), "billing") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want it to name the step and keep the cause", err)
	}
}
```

## Stage 3 — docs

`README.md`: add a section:

```markdown
## Capture — the tables a migration list creates

`ddl.Step{Name, Run}` names one module's migration (`Run` has the signature of every module's `Migrate`).
`ddl.Capture(steps)` runs them against a recorder that touches no database and returns every created table,
in order, with its step — what `webtyp.com/erd` draws as an app's database diagram.
```

## Delivery

- `gotest` green (also the wasm pass). Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`,
  `GOFLAGS=-mod=mod`. A PR that does not compile, or whose tests could not run, is a failed delivery.
- `grep -n "map\[" capture.go` → empty.
- `git diff --stat main` lists exactly: `capture.go`, `capture_test.go`, `README.md`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `capture.go` | `Step`, `Table`, `Capture`, `recorder` |
| 2 | `capture_test.go` | order and step names; a failing step is named |
| 3 | `README.md` | one section |
