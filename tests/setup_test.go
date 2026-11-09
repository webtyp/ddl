package ddl_test

import (
	"webtyp.com/ddl"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// tableCols is one table's columns in the fake database (no map: AGENTS.md).
type tableCols struct {
	table string
	cols  []string
}

// fakeConn lists columns per table and returns values for every probe query.
type fakeConn struct {
	tables   []tableCols
	colsErr  error
	values   []any // what every probe query returns, one row per value
	queryErr error
	queries  int // probe queries run
}

func (c *fakeConn) Exec(string, ...any) (storage.Result, error) { return storage.Affected(0), nil }
func (c *fakeConn) Compile(q storage.Query, m model.Model) (storage.Plan, error) {
	return storage.Plan{Query: "probe " + q.Table + "." + q.Columns[0]}, nil
}
func (c *fakeConn) Query(string, ...any) (storage.Rows, error) {
	c.queries++
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	return &fakeRows{values: c.values}, nil
}
func (c *fakeConn) TableColumns(table string) ([]string, error) {
	if c.colsErr != nil {
		return nil, c.colsErr
	}
	for _, t := range c.tables {
		if t.table == table {
			return t.cols, nil
		}
	}
	return nil, nil
}

// execOnly has Exec and nothing else: it cannot probe.
type execOnly struct{}

func (execOnly) Exec(string, ...any) (storage.Result, error) { return storage.Affected(0), nil }

type fakeRows struct {
	values []any
	i      int
}

func (r *fakeRows) Next() bool {
	if r.i < len(r.values) {
		r.i++
		return true
	}
	return false
}
func (r *fakeRows) Scan(dest ...any) error     { *(dest[0].(*any)) = r.values[r.i-1]; return nil }
func (r *fakeRows) Columns() ([]string, error) { return nil, nil }
func (r *fakeRows) Close() error               { return nil }
func (r *fakeRows) Err() error                 { return nil }

// recordingCompiler is a ddl.Compiler that keeps every statement.
type recordingCompiler struct{ stmts []ddl.Stmt }

func (c *recordingCompiler) CompileDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	c.stmts = append(c.stmts, s)
	return "ddl", nil, nil
}

// testModel is a model with a table name and fields; renamed fills OldNames.
type testModel struct {
	table  string
	fields []model.Field
}

func (m *testModel) ModelName() string              { return m.table }
func (m *testModel) Schema() []model.Field          { return m.fields }
func (m *testModel) Pointers() []any                { return nil }
func (m *testModel) IsNil() bool                    { return m == nil }
func (m *testModel) EncodeFields(model.FieldWriter) {}
func (m *testModel) DecodeFields(model.FieldReader) {}

type renamedModel struct {
	testModel
	oldNames map[string]string
}

func (m *renamedModel) OldNames() map[string]string { return m.oldNames }

func calendarConfig() *testModel {
	return &testModel{table: "work_calendar_config", fields: []model.Field{{Name: "id", Type: model.Text()}}}
}
