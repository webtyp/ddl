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
	if err := conn.Exec(plan.Query, plan.Args...); err != nil {
		t.Fatalf("insert: %v", err)
	}
}
