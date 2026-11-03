package ddl

import "webtyp.com/storage"

// Snapshot is a database opened from a backup file, read-only: which tables and
// columns it holds (SchemaInspector) and the rows of each table (ReadTable).
// Close releases it — an engine that restored the backup into a temporary
// database drops that database.
//
// Example:
//
//	rows, err := snap.ReadTable("staff")
type Snapshot interface {
	SchemaInspector
	// ReadTable returns every row of table, with every column, in the
	// engine's natural order, each value as text: scan into *any and get a
	// string, or nil for NULL (storage.ScanAny keeps nil only for *any). The engine writes the query
	// (quoting and casts included), so the caller never writes dialect SQL
	// and reads every engine the same way.
	ReadTable(table string) (storage.Rows, error)
	// Notices are what the engine tolerated on purpose while opening the backup, each one a plain sentence for a
	// beginner: what happened, whether the data is affected, what to do. Each is built with webtyp.com/fmt.Err so
	// langc extracts its words and the engine's lang.json translates them. Empty when nothing happened.
	Notices() []error
	Close() error
}

// BackupFormat opens the backup files of one storage engine. Each engine knows
// its own format (postgres restores a pg_dump archive into a temporary
// database; sqlite opens the file itself), so a consumer reads the backup of
// any engine through the same Snapshot.
//
// Example:
//
//	snap, err := format.OpenBackup("migrate/pastdb/1793656810107289500-db.backup")
//	if err != nil { return err }
//	defer snap.Close()
type BackupFormat interface {
	OpenBackup(path string) (Snapshot, error)
}
