package ddl_test

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/storage/mock"
)

func TestCapture_ListsEveryCreatedTableInOrderWithItsStep(t *testing.T) {
	steps := []ddl.Step{
		{Name: "auth", Run: func(conn ddl.Execer, c ddl.Compiler) error {
			return ddl.New(conn, c).Sync(&mock.Model{Table: "tenant"})
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
