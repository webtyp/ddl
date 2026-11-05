package ddl

import (
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

const (
	// LeftoverRowCap is how many rows Leftovers counts per column; beyond it, Leftover.More is true.
	LeftoverRowCap = 100
	// LeftoverSamples is how many example values Leftovers returns per column.
	LeftoverSamples = 3

	errLeftoversNoProbe = "ddl: leftovers: the connection cannot list columns or read data"
	errDropColumn       = "ddl: drop column:"
	errCopyColumn       = "ddl: copy column:"
	errTableAndColumn   = "table and column"
	errTableColumnMoved = "table, column and moved-to"
	errRequired         = "are required"
)

// Removed is a column a model no longer has, and where its data went. An app declares it; webtyp dev writes it.
//
//	ddl.Removed{Table: "work_calendar_config", Column: "timezone", Note: "moved to config.App.Zone"}
type Removed struct {
	Table   string // table that still has the column
	Column  string // the column the model no longer has
	MovedTo string // column of the same table that receives the data (CopyColumn); "" = none
	Note    string // where the data went, in plain words; "" = nobody said
}

// Leftover is a column the database keeps although the model no longer has it, because it holds data.
type Leftover struct {
	Table   string
	Column  string
	Rows    int      // rows with a value, counted up to LeftoverRowCap
	More    bool     // true when there are more than LeftoverRowCap
	Samples []string // up to LeftoverSamples values, in the order the database returns them
	Removed Removed  // the declaration for this column; zero value when there is none
}

// prober is the DML half Sync and Leftovers need to ask whether a column holds data.
type prober interface {
	storage.Compiler
	Query(query string, args ...any) (storage.Rows, error)
}

// Leftovers lists, read-only, the columns of the given tables that the models no longer have but still hold data.
// tables usually comes from Capture; a table listed twice is read once.
//
//	tables, _ := ddl.Capture(steps)
//	left, err := ddl.New(conn, compiler).Leftovers(tables, config.Removed)
func (d *DB) Leftovers(tables []Table, removed []Removed) ([]Leftover, error) {
	introspector, canList := d.conn.(TableIntrospector)
	p, canProbe := d.conn.(prober)
	if !canList || !canProbe {
		return nil, fmt.Err(errLeftoversNoProbe)
	}
	var out []Leftover
	var seen []string
	for _, t := range tables {
		name := t.Model.ModelName()
		if contains(seen, name) {
			continue
		}
		seen = append(seen, name)
		cols, err := introspector.TableColumns(name)
		if err != nil {
			return nil, fmt.Errf("%s: list columns: %v", name, err)
		}
		var oldNames map[string]string
		if rp, ok := t.Model.(RenameProvider); ok {
			oldNames = rp.OldNames()
		}
		for _, col := range cols {
			if schemaHasColumn(t.Model.Schema(), col) || isRenameSource(oldNames, col) {
				continue
			}
			l, err := probeColumn(p, t.Model, name, col)
			if err != nil {
				return nil, err
			}
			if l.Rows == 0 {
				continue
			}
			l.Removed = findRemoved(removed, name, col)
			out = append(out, l)
		}
	}
	return out, nil
}

// probeColumn reads at most LeftoverRowCap+1 non-NULL values of col.
func probeColumn(p prober, m model.Model, table, col string) (Leftover, error) {
	plan, err := p.Compile(storage.Query{
		Action:     storage.ActionReadAll,
		Table:      table,
		Columns:    []string{col},
		Conditions: []storage.Condition{storage.IsNotNull(col)},
		Limit:      LeftoverRowCap + 1,
	}, m)
	if err != nil {
		return Leftover{}, fmt.Errf("%s: read column %s: %v", table, col, err)
	}
	rows, err := p.Query(plan.Query, plan.Args...)
	if err != nil {
		return Leftover{}, fmt.Errf("%s: read column %s: %v", table, col, err)
	}
	defer rows.Close()
	l := Leftover{Table: table, Column: col}
	for rows.Next() {
		if l.Rows == LeftoverRowCap {
			l.More = true
			break
		}
		l.Rows++
		if len(l.Samples) < LeftoverSamples {
			var v any
			if err := rows.Scan(&v); err != nil {
				return Leftover{}, fmt.Errf("%s: read column %s: %v", table, col, err)
			}
			l.Samples = append(l.Samples, fmt.Convert(v).String())
		}
	}
	if err := rows.Err(); err != nil {
		return Leftover{}, fmt.Errf("%s: read column %s: %v", table, col, err)
	}
	return l, nil
}

func findRemoved(removed []Removed, table, col string) Removed {
	for _, r := range removed {
		if r.Table == table && r.Column == col {
			return r
		}
	}
	return Removed{}
}

// DropColumn drops one column, data included. Only for a decision a person made (webtyp dev's «yes»).
//
//	err := db.DropColumn("work_calendar_config", "timezone")
func (d *DB) DropColumn(table, column string) error {
	if table == "" || column == "" {
		return fmt.Err(errDropColumn, errTableAndColumn, errRequired)
	}
	return d.execDDL(Stmt{Op: OpDropColumn, Table: table, ColumnName: column}, nil)
}

// CopyColumn copies r.Column into r.MovedTo on the rows where r.MovedTo is empty, before the column is dropped.
//
//	err := db.CopyColumn(ddl.Removed{Table: "holiday", Column: "label", MovedTo: "name"})
func (d *DB) CopyColumn(r Removed) error {
	if r.Table == "" || r.Column == "" || r.MovedTo == "" {
		return fmt.Err(errCopyColumn, errTableColumnMoved, errRequired)
	}
	return d.execDDL(Stmt{Op: OpCopyColumn, Table: r.Table, OldName: r.Column, ColumnName: r.MovedTo}, nil)
}
