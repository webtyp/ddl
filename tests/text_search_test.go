package ddl_test

import (
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/storage"
	"webtyp.com/storage/mem"
)

// searchKind is a text kind that enters text_search (webtyp.com/input does this for Text, Phone, Email, Rut).
type searchKind struct{ model.Kind }

func (searchKind) SearchableText() {}

// memDDL is storage/mem with DDL accepted as a no-op (mem creates a table on its first insert), so Sync's
// DML half — the text_search rebuild — runs for real.
type memDDL struct{ storage.Conn }

func (c memDDL) Exec(query string, args ...any) (storage.Result, error) {
	if query == "ddl" { // what recordingCompiler compiles every DDL statement to
		return storage.Affected(0), nil
	}
	return c.Conn.Exec(query, args...)
}

func patientFields(withPhone bool) []model.Field {
	f := []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: searchKind{model.Text()}},
		{Name: "phone", Type: model.Text()},
	}
	if withPhone {
		f[2].Type = searchKind{model.Text()}
	}
	return f
}

// insertPatient writes a row as an older version of the app did: without text_search.
func insertPatient(t *testing.T, conn storage.Conn, id, name, phone string) {
	t.Helper()
	m := &testModel{table: "patient", fields: patientFields(false)}
	plan, err := conn.Compile(storage.Query{Action: storage.ActionCreate, Table: "patient", Columns: []string{"id", "name", "phone"}, Values: []any{id, name, phone}}, m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(plan.Query, plan.Args...); err != nil {
		t.Fatal(err)
	}
}

func textSearchOfPatient(t *testing.T, conn storage.Conn, id string) string {
	t.Helper()
	m := &testModel{table: "patient", fields: patientFields(false)}
	plan, err := conn.Compile(storage.Query{Action: storage.ActionReadOne, Table: "patient", Columns: []string{model.FieldTextSearch}, Conditions: []storage.Condition{storage.Eq("id", id)}, Limit: 1}, m)
	if err != nil {
		t.Fatal(err)
	}
	var got string
	if err := conn.QueryRow(plan.Query, plan.Args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTextSearch_SyncRebuildsExistingRows(t *testing.T) {
	conn := memDDL{mem.New()}
	insertPatient(t, conn, "p1", "Lucía Pérez", "+56911111111")
	if err := ddl.New(conn, &recordingCompiler{}).Sync(&testModel{table: "patient", fields: patientFields(false)}); err != nil {
		t.Fatal(err)
	}
	if got := textSearchOfPatient(t, conn, "p1"); got != "lucia perez" {
		t.Errorf("text_search = %q, want %q", got, "lucia perez")
	}
}

func TestTextSearch_SyncAddsTheColumn(t *testing.T) {
	compiler := &recordingCompiler{}
	if err := ddl.New(memDDL{mem.New()}, compiler).Sync(&testModel{table: "patient", fields: patientFields(false)}); err != nil {
		t.Fatal(err)
	}
	for _, s := range compiler.stmts {
		if s.Op == ddl.OpAddColumn && s.Column.Name == model.FieldTextSearch {
			return
		}
	}
	t.Errorf("no ADD COLUMN %s in %+v", model.FieldTextSearch, compiler.stmts)
}

func TestTextSearch_SyncRebuildsWhenTheFieldsChange(t *testing.T) {
	conn := memDDL{mem.New()}
	insertPatient(t, conn, "p1", "Ana", "+56911111111")
	db := ddl.New(conn, &recordingCompiler{})
	if err := db.Sync(&testModel{table: "patient", fields: patientFields(false)}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(&testModel{table: "patient", fields: patientFields(true)}); err != nil {
		t.Fatal(err)
	}
	if got := textSearchOfPatient(t, conn, "p1"); got != "ana 56911111111" {
		t.Errorf("text_search = %q, want %q", got, "ana 56911111111")
	}
}

func TestTextSearch_SyncDoesNotRebuildWhenNothingChanged(t *testing.T) {
	conn := memDDL{mem.New()}
	insertPatient(t, conn, "p1", "Ana", "")
	db := ddl.New(conn, &recordingCompiler{})
	m := &testModel{table: "patient", fields: patientFields(false)}
	if err := db.Sync(m); err != nil {
		t.Fatal(err)
	}
	// a write that bypassed the framework leaves a value Sync must not overwrite when the fields did not change
	plan, _ := conn.Compile(storage.Query{Action: storage.ActionUpdate, Table: "patient", Columns: []string{model.FieldTextSearch}, Values: []any{"kept"}, Conditions: []storage.Condition{storage.Eq("id", "p1")}}, m)
	if _, err := conn.Exec(plan.Query, plan.Args...); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(m); err != nil {
		t.Fatal(err)
	}
	if got := textSearchOfPatient(t, conn, "p1"); got != "kept" {
		t.Errorf("text_search = %q, want kept (no rebuild)", got)
	}
}

func TestTextSearch_SyncWithoutReadsFails(t *testing.T) {
	err := ddl.New(execOnly{}, &recordingCompiler{}).Sync(&testModel{table: "patient", fields: patientFields(false)})
	want := "ddl: text_search of this table must be rebuilt, and the connection cannot read rows; run the migration with a storage.Conn: patient"
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestTextSearch_CaptureRecordsOnlyTheModelTable(t *testing.T) {
	m := &testModel{table: "patient", fields: patientFields(false)}
	tables, err := ddl.Capture([]ddl.Step{{Name: "patient_directory", Run: func(conn ddl.Execer, c ddl.Compiler) error {
		return ddl.New(conn, c).Sync(m)
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Model.ModelName() != "patient" {
		t.Errorf("tables = %+v, want only patient", tables)
	}
}
