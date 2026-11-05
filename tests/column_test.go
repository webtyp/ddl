package ddl_test

import (
	"testing"

	"webtyp.com/ddl"
)

func TestDropColumn_EmitsDropColumn(t *testing.T) {
	comp := &recordingCompiler{}
	db := ddl.New(execOnly{}, comp)
	if err := db.DropColumn("work_calendar_config", "timezone"); err != nil {
		t.Fatal(err)
	}
	want := ddl.Stmt{Op: ddl.OpDropColumn, Table: "work_calendar_config", ColumnName: "timezone"}
	if len(comp.stmts) != 1 || comp.stmts[0] != want {
		t.Fatalf("want %+v, got %+v", want, comp.stmts)
	}
}

func TestDropColumn_EmptyNameFails(t *testing.T) {
	comp := &recordingCompiler{}
	db := ddl.New(execOnly{}, comp)
	err := db.DropColumn("work_calendar_config", "")
	if err == nil || err.Error() != "ddl: drop column: table and column are required" || len(comp.stmts) != 0 {
		t.Fatalf("got %v, %v", err, comp.stmts)
	}
}

func TestCopyColumn_EmitsCopyColumn(t *testing.T) {
	comp := &recordingCompiler{}
	db := ddl.New(execOnly{}, comp)
	if err := db.CopyColumn(ddl.Removed{Table: "holiday", Column: "label", MovedTo: "name"}); err != nil {
		t.Fatal(err)
	}
	want := ddl.Stmt{Op: ddl.OpCopyColumn, Table: "holiday", OldName: "label", ColumnName: "name"}
	if len(comp.stmts) != 1 || comp.stmts[0] != want {
		t.Fatalf("want %+v, got %+v", want, comp.stmts)
	}
}

func TestCopyColumn_WithoutMovedToFails(t *testing.T) {
	comp := &recordingCompiler{}
	db := ddl.New(execOnly{}, comp)
	err := db.CopyColumn(ddl.Removed{Table: "holiday", Column: "label"})
	if err == nil || err.Error() != "ddl: copy column: table, column and moved-to are required" || len(comp.stmts) != 0 {
		t.Fatalf("got %v, %v", err, comp.stmts)
	}
}
