package ddl_test

import (
	"errors"
	"strings"
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/fmt"
	"webtyp.com/model"
)

func leftoverSetup() (*fakeConn, *ddl.DB, ddl.Table) {
	conn := &fakeConn{tables: []tableCols{{"work_calendar_config", []string{"id", "timezone"}}}}
	return conn, ddl.New(conn, &recordingCompiler{}), ddl.Table{Step: "business_calendar", Model: calendarConfig()}
}

func TestLeftovers_ColumnWithDataIsListedWithRowsAndSamples(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	conn.values = []any{"America/Santiago"}
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 leftover, got %v", got)
	}
	l := got[0]
	if l.Table != "work_calendar_config" || l.Column != "timezone" || l.Rows != 1 || l.More ||
		len(l.Samples) != 1 || l.Samples[0] != "America/Santiago" || l.Removed != (ddl.Removed{}) {
		t.Fatalf("unexpected leftover %+v", l)
	}
}

func TestLeftovers_EmptyColumnIsNotListed(t *testing.T) {
	_, db, tbl := leftoverSetup()
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("want none, got %v, %v", got, err)
	}
}

func TestLeftovers_CountStopsAtTheCap(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	for i := 0; i < 150; i++ {
		conn.values = append(conn.values, fmt.Sprintf("v%d", i))
	}
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err != nil {
		t.Fatal(err)
	}
	l := got[0]
	if l.Rows != ddl.LeftoverRowCap || !l.More {
		t.Fatalf("want %d rows and More, got %d %v", ddl.LeftoverRowCap, l.Rows, l.More)
	}
	if len(l.Samples) != 3 || l.Samples[0] != "v0" || l.Samples[1] != "v1" || l.Samples[2] != "v2" {
		t.Fatalf("samples %v", l.Samples)
	}
}

func TestLeftovers_DeclarationIsAttached(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	conn.values = []any{"America/Santiago"}
	removed := []ddl.Removed{
		{Table: "holiday", Column: "label", Note: "x"},
		{Table: "work_calendar_config", Column: "timezone", Note: "moved to config.App.Zone"},
	}
	got, err := db.Leftovers([]ddl.Table{tbl}, removed)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Removed != removed[1] {
		t.Fatalf("want %+v, got %+v", removed[1], got[0].Removed)
	}
}

func TestLeftovers_RenameSourceIsNotListed(t *testing.T) {
	conn, db, _ := leftoverSetup()
	conn.values = []any{"x"}
	m := &renamedModel{
		testModel: testModel{table: "work_calendar_config", fields: []model.Field{
			{Name: "id", Type: model.Text()}, {Name: "zone", Type: model.Text()},
		}},
		oldNames: map[string]string{"zone": "timezone"},
	}
	got, err := db.Leftovers([]ddl.Table{{Step: "s", Model: m}}, nil)
	if err != nil || len(got) != 0 || conn.queries != 0 {
		t.Fatalf("want none and no query, got %v, %v, %d queries", got, err, conn.queries)
	}
}

func TestLeftovers_SameTableTwiceIsReadOnce(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	conn.values = []any{"x"}
	got, err := db.Leftovers([]ddl.Table{tbl, tbl}, nil)
	if err != nil || len(got) != 1 || conn.queries != 1 {
		t.Fatalf("want 1 leftover and 1 query, got %v, %v, %d", got, err, conn.queries)
	}
}

func TestLeftovers_ConnectionWithoutProbeFails(t *testing.T) {
	_, _, tbl := leftoverSetup()
	db := ddl.New(execOnly{}, &recordingCompiler{})
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err == nil || err.Error() != "ddl: leftovers: the connection cannot list columns or read data" || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestLeftovers_ListColumnsErrorStops(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	conn.colsErr = errors.New("down")
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err == nil || got != nil || !strings.Contains(err.Error(), "work_calendar_config") || !strings.Contains(err.Error(), "down") {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestLeftovers_ReadErrorStops(t *testing.T) {
	conn, db, tbl := leftoverSetup()
	conn.queryErr = errors.New("down")
	got, err := db.Leftovers([]ddl.Table{tbl}, nil)
	if err == nil || got != nil {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, w := range []string{"work_calendar_config", "timezone", "down"} {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("%q missing in %q", w, err.Error())
		}
	}
}
