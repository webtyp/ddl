package ddl

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

const (
	// textSearchStateTable is where ddl remembers, per table, the fields its text_search was built from, so a
	// migration rebuilds the column only when it is new or that list changed (RECORD_SEARCH D2).
	textSearchStateTable = "text_search_state"
	textSearchStateKey   = "table_name"
	textSearchStateValue = "fields"

	errTextSearchNoReads = "ddl: text_search of this table must be rebuilt, and the connection cannot read rows; run the migration with a storage.Conn:"
)

var textSearchStateFields = []model.Field{
	{Name: textSearchStateKey, Type: model.Text(), DB: &model.FieldDB{PK: true}},
	{Name: textSearchStateValue, Type: model.Text(), NotNull: true},
}

// recordsOnly is implemented by the Capture recorder: a connection that touches no database has no rows to rebuild.
type recordsOnly interface{ recordsOnly() }

// withTextSearchColumn is the schema Sync reconciles: the model's fields plus text_search when any field enters it.
func withTextSearchColumn(schema []model.Field) []model.Field {
	if len(model.TextSearchFields(schema)) == 0 {
		return schema
	}
	out := make([]model.Field, len(schema), len(schema)+1)
	copy(out, schema)
	return append(out, model.Field{Name: model.FieldTextSearch, Type: model.Text(), OmitEmpty: true})
}

// textSearchFieldList is the list text_search is built from, as ddl stores it: field names joined by ",".
func textSearchFieldList(schema []model.Field) string {
	idx := model.TextSearchFields(schema)
	names := make([]string, len(idx))
	for j, i := range idx {
		names[j] = schema[i].Name
	}
	return fmt.JoinSlice(names, ",")
}

// syncTextSearch rebuilds text_search of every row of m's table when the column is new or the fields that enter
// it changed since the last migration, and records the new list. A table without such fields does nothing.
func (d *DB) syncTextSearch(m model.Model) error {
	want := textSearchFieldList(m.Schema())
	if want == "" {
		return nil
	}
	if _, dry := d.conn.(recordsOnly); dry {
		return nil
	}
	conn, ok := d.conn.(storage.Conn)
	if !ok {
		return fmt.Err(errTextSearchNoReads, m.ModelName())
	}
	state := &schemaModel{name: textSearchStateTable, fields: textSearchStateFields}
	if err := d.createTable(state); err != nil {
		return err
	}
	byTable := []storage.Condition{storage.Eq(textSearchStateKey, m.ModelName())}
	plan, err := conn.Compile(storage.Query{Action: storage.ActionReadAll, Table: textSearchStateTable, Columns: []string{textSearchStateValue}, Conditions: byTable, Limit: 1}, state)
	if err != nil {
		return err
	}
	rows, err := conn.Query(plan.Query, plan.Args...)
	if err != nil {
		return err
	}
	var stored string
	found := rows.Next()
	if found {
		if err := rows.Scan(&stored); err != nil {
			rows.Close()
			return err
		}
	}
	rows.Close()
	if found && stored == want {
		return nil
	}
	if err := storage.RewriteTextSearch(conn, m, nil, nil); err != nil {
		return err
	}
	q := storage.Query{Action: storage.ActionCreate, Table: textSearchStateTable, Columns: []string{textSearchStateKey, textSearchStateValue}, Values: []any{m.ModelName(), want}}
	if found {
		q = storage.Query{Action: storage.ActionUpdate, Table: textSearchStateTable, Columns: []string{textSearchStateValue}, Values: []any{want}, Conditions: byTable}
	}
	plan, err = conn.Compile(q, state)
	if err != nil {
		return err
	}
	_, err = conn.Exec(plan.Query, plan.Args...)
	return err
}
