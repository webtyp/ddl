package conformance

import (
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
	"webtyp.com/storage/conformance"
)

type Factory struct {
	Name string
	// New returns a fresh ddl.DB plus the storage.Conn it writes through (for introspection/DML
	// verification) and an introspector to read back the resulting schema. Called once per clause.
	New func(t *testing.T) (schema *ddl.DB, conn storage.Conn, cols func(table string) []string)
}

func Run(t *testing.T, f Factory) {
	t.Run("create_table_makes_expected_columns", func(t *testing.T) {
		schema, _, cols := f.New(t)
		w := &conformance.Widget{}
		err := schema.Sync(w)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
		c := cols(w.ModelName())
		// Expected columns come from the model itself: a field added to Widget can never make this list stale.
		var expected []string
		for _, f := range conformance.WidgetModel.Fields {
			expected = append(expected, f.Name)
		}
		if len(c) != len(expected) {
			t.Fatalf("expected columns %v, got %v", expected, c)
		}
		for _, col := range expected {
			found := false
			for _, x := range c {
				if x == col {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("column %s not found in schema columns %v", col, c)
			}
		}
	})

	t.Run("create_table_is_idempotent", func(t *testing.T) {
		schema, _, _ := f.New(t)
		w := &conformance.Widget{}
		err := schema.Sync(w)
		if err != nil {
			t.Fatalf("first Sync failed: %v", err)
		}
		err = schema.Sync(w)
		if err != nil {
			t.Fatalf("second Sync (idempotency) failed: %v", err)
		}
	})

	t.Run("sync_adds_new_column", func(t *testing.T) {
		schema, _, cols := f.New(t)
		w := &conformance.Widget{}
		err := schema.Sync(w)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		// Now we'll sync with an extra column
		fields := append(w.Schema(), model.Field{
			Name: "extra",
			Type: model.Text(),
		})

		err = schema.SyncSchema(w.ModelName(), fields)
		if err != nil {
			t.Fatalf("SyncSchema with extra field failed: %v", err)
		}

		c := cols(w.ModelName())
		found := false
		for _, col := range c {
			if col == "extra" {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected column 'extra' to be added by SyncSchema, but columns are %v", c)
		}
	})

	t.Run("drop_table_removes_schema", func(t *testing.T) {
		schema, _, cols := f.New(t)
		w := &conformance.Widget{}
		err := schema.Sync(w)
		if err != nil {
			t.Fatalf("Sync failed: %v", err)
		}

		err = schema.DropTable(w)
		if err != nil {
			t.Fatalf("DropTable failed: %v", err)
		}

		c := cols(w.ModelName())
		if len(c) > 0 {
			t.Fatalf("expected table to have no columns (or not exist), got columns: %v", c)
		}
	})

	t.Run("copy_column_fills_only_empty_rows", func(t *testing.T) {
		schema, conn, _ := f.New(t)
		w := &conformance.Widget{}
		if err := schema.Sync(w); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
		insertWidget(t, conn, "a", "x")
		if err := schema.SyncSchema(w.ModelName(), append(w.Schema(), model.Field{Name: "label", Type: model.Text()})); err != nil {
			t.Fatalf("SyncSchema with label failed: %v", err)
		}
		if err := schema.CopyColumn(ddl.Removed{Table: w.ModelName(), Column: "note", MovedTo: "label"}); err != nil {
			t.Fatalf("CopyColumn failed: %v", err)
		}
		plan, err := conn.Compile(storage.Query{
			Action:     storage.ActionReadAll,
			Table:      w.ModelName(),
			Columns:    []string{"label"},
			Conditions: []storage.Condition{storage.IsNotNull("label")},
		}, w)
		if err != nil {
			t.Fatalf("compile read: %v", err)
		}
		rows, err := conn.Query(plan.Query, plan.Args...)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		defer rows.Close()
		var got []string
		for rows.Next() {
			var v any
			if err := rows.Scan(&v); err != nil {
				t.Fatalf("scan: %v", err)
			}
			got = append(got, fmt.Convert(v).String())
		}
		if len(got) != 1 || got[0] != "x" {
			t.Fatalf("expected label [x] after CopyColumn, got %v", got)
		}
	})

	t.Run("text_search_is_built_and_found", func(t *testing.T) {
		schema, conn, _ := f.New(t)
		p := &searchPerson{}
		if err := schema.Sync(p); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
		insertPerson(t, conn, "p1", "Luc\u00eda P\u00e9rez", "+56 9 1234 5678")
		insertPerson(t, conn, "p2", "Bruno D\u00edaz", "+56 9 8765 4321")
		if err := storage.RewriteTextSearch(conn, p, nil, nil); err != nil {
			t.Fatalf("RewriteTextSearch failed: %v", err)
		}
		for _, c := range []struct{ typed, want string }{
			{"lucia", "p1"},       // accent and case folded
			{"56912345678", "p1"}, // digits of a phone typed without spaces
			{"diaz", "p2"},
		} {
			if got := searchPersonIDs(t, conn, c.typed); len(got) != 1 || got[0] != c.want {
				t.Errorf("search %q = %v, want [%s]", c.typed, got, c.want)
			}
		}
	})

	t.Run("drop_column_drops_a_column_with_data", func(t *testing.T) {
		schema, conn, cols := f.New(t)
		w := &conformance.Widget{}
		if err := schema.Sync(w); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
		insertWidget(t, conn, "a", "x")
		if err := schema.DropColumn(w.ModelName(), "note"); err != nil {
			t.Fatalf("DropColumn failed: %v", err)
		}
		for _, c := range cols(w.ModelName()) {
			if c == "note" {
				t.Fatalf("column note still present after DropColumn")
			}
		}
	})
}

// insertWidget writes one Widget row through the DML half of conn.
func insertWidget(t *testing.T, conn storage.Conn, id, note string) {
	t.Helper()
	w := &conformance.Widget{}
	plan, err := conn.Compile(storage.Query{
		Action:  storage.ActionCreate,
		Table:   w.ModelName(),
		Columns: []string{"id", "name", "qty", "active", "note"},
		Values:  []any{id, "n", 1, true, note},
	}, w)
	if err != nil {
		t.Fatalf("compile insert: %v", err)
	}
	if _, err := conn.Exec(plan.Query, plan.Args...); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// searchKind is a text kind that enters text_search (webtyp.com/input does this for Text, Phone, Email, Rut).
type searchKind struct{ model.Kind }

func (searchKind) SearchableText() {}

// searchPerson is a table with two fields in text_search.
type searchPerson struct{ ID, Name, Phone string }

func (p *searchPerson) ModelName() string { return "search_person" }
func (p *searchPerson) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "name", Type: searchKind{model.Text()}},
		{Name: "phone", Type: searchKind{model.Text()}},
	}
}
func (p *searchPerson) Pointers() []any                { return []any{&p.ID, &p.Name, &p.Phone} }
func (p *searchPerson) IsNil() bool                    { return p == nil }
func (p *searchPerson) EncodeFields(model.FieldWriter) {}
func (p *searchPerson) DecodeFields(model.FieldReader) {}

// insertPerson writes a row as an older version of the app did: without text_search.
func insertPerson(t *testing.T, conn storage.Conn, id, name, phone string) {
	t.Helper()
	plan, err := conn.Compile(storage.Query{Action: storage.ActionCreate, Table: "search_person", Columns: []string{"id", "name", "phone"}, Values: []any{id, name, phone}}, &searchPerson{})
	if err != nil {
		t.Fatalf("compile insert: %v", err)
	}
	if _, err := conn.Exec(plan.Query, plan.Args...); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// searchPersonIDs finds rows the way crud does: one Like per typed word on text_search.
func searchPersonIDs(t *testing.T, conn storage.Conn, typed string) []string {
	t.Helper()
	plan, err := conn.Compile(storage.Query{Action: storage.ActionReadAll, Table: "search_person", Columns: []string{"id"}, Conditions: []storage.Condition{storage.Like(model.FieldTextSearch, "%"+typed+"%")}}, &searchPerson{})
	if err != nil {
		t.Fatalf("compile search: %v", err)
	}
	rows, err := conn.Query(plan.Query, plan.Args...)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}
