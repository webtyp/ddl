package ddl_test

import (
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/storage"
)

// memSnapshot is the smallest Snapshot: what a consumer's test injects instead
// of a real engine.
type memSnapshot struct{ closed bool }

func (s *memSnapshot) Tables() ([]string, error) { return []string{"staff"}, nil }
func (s *memSnapshot) Columns(string) ([]ddl.ColumnInfo, error) {
	return []ddl.ColumnInfo{{Name: "id", PK: true}}, nil
}
func (s *memSnapshot) ReadTable(string) (storage.Rows, error) { return nil, nil }
func (s *memSnapshot) Close() error                           { s.closed = true; return nil }

type memFormat struct{ snap *memSnapshot }

func (f memFormat) OpenBackup(string) (ddl.Snapshot, error) { return f.snap, nil }

func TestBackupFormatOpensASnapshotThatCloses(t *testing.T) {
	var format ddl.BackupFormat = memFormat{snap: &memSnapshot{}}
	snap, err := format.OpenBackup("db.backup")
	if err != nil {
		t.Fatalf("OpenBackup: %v", err)
	}
	tables, err := snap.Tables()
	if err != nil || len(tables) != 1 || tables[0] != "staff" {
		t.Fatalf("Tables() = %v, %v; want [staff]", tables, err)
	}
	if err := snap.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !format.(memFormat).snap.closed {
		t.Fatal("Close did not release the snapshot")
	}
}
