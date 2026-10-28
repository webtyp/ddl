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
