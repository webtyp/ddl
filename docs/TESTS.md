# Tests — webtyp/ddl

## Sync ([diagram](diagrams/SYNC_FLOW.md))
| Rule it protects | Test |
|---|---|
| Without listing columns, every field is added (failures only logged) | [TestSync_NoIntrospector](../ddl_test.go) |
| A table that already matches gets only CREATE TABLE IF NOT EXISTS | [TestSync_WithIntrospector_NoChanges](../ddl_test.go) |
| A field the table lacks is added | [TestSync_WithIntrospector_AddColumn](../ddl_test.go) |
| A field declared with its old name is renamed, not added | [TestSync_WithIntrospector_Rename](../ddl_test.go) |
| A column the model lost that holds data is kept | [TestSync_WithIntrospector_SafeDrop_WithData](../ddl_test.go) |
| A column the model lost that is empty is dropped | [TestSync_WithIntrospector_SafeDrop_NoData](../ddl_test.go) |
| With transactions, every model runs in one | [TestSync_Transaction](../ddl_test.go) |
| The transaction keeps the column listing | [TestSync_Transaction_WithIntrospector](../ddl_test.go) |
| A failing statement rolls everything back | [TestSync_Transaction_Rollback](../ddl_test.go) |
| A DDL-only connection never drops blind | [TestSync_DDLOnlyConn_SkipsSafeDrop](../ddl_test.go) |
| `New` accepts a connection that only runs DDL | [TestNew_AcceptsDDLOnlyConn](../ddl_test.go) |
| `DropTable` emits one OpDropTable | [TestDropTable](../ddl_test.go) |
| `CreateDatabase` emits one OpCreateDatabase | [TestCreateDatabase](../ddl_test.go) |

## Leftovers ([diagram](diagrams/LEFTOVERS_FLOW.md))
| Rule it protects | Test |
|---|---|
| A column the model lost that holds data is listed with its rows and examples | [TestLeftovers_ColumnWithDataIsListedWithRowsAndSamples](../tests/leftovers_test.go) |
| An empty column is not a leftover (`Sync` drops it) | [TestLeftovers_EmptyColumnIsNotListed](../tests/leftovers_test.go) |
| Counting stops at `LeftoverRowCap`, `More` says there are more; 3 samples | [TestLeftovers_CountStopsAtTheCap](../tests/leftovers_test.go) |
| The app's `Removed` for that table and column travels with the leftover | [TestLeftovers_DeclarationIsAttached](../tests/leftovers_test.go) |
| The old name of a renamed field is never listed nor read | [TestLeftovers_RenameSourceIsNotListed](../tests/leftovers_test.go) |
| A table listed by two steps is read once | [TestLeftovers_SameTableTwiceIsReadOnce](../tests/leftovers_test.go) |
| Without listing columns and reading data there is no evidence: error | [TestLeftovers_ConnectionWithoutProbeFails](../tests/leftovers_test.go) |
| Listing columns fails → error naming the table, nothing returned | [TestLeftovers_ListColumnsErrorStops](../tests/leftovers_test.go) |
| Reading a column fails → error naming table and column | [TestLeftovers_ReadErrorStops](../tests/leftovers_test.go) |

## One column, by a person's decision
| Rule it protects | Test |
|---|---|
| `DropColumn` emits one OpDropColumn | [TestDropColumn_EmitsDropColumn](../tests/column_test.go) |
| `DropColumn` without a table or column fails before any statement | [TestDropColumn_EmptyNameFails](../tests/column_test.go) |
| `CopyColumn` emits OpCopyColumn (source in `OldName`, destination in `ColumnName`) | [TestCopyColumn_EmitsCopyColumn](../tests/column_test.go) |
| `CopyColumn` without a destination fails before any statement | [TestCopyColumn_WithoutMovedToFails](../tests/column_test.go) |

Backends prove `CopyColumn` (only empty rows) and `DropColumn` (column with data) in `conformance.Run`.

## Capture and backups
| Rule it protects | Test |
|---|---|
| Every created table is listed in order with its step | [TestCapture_ListsEveryCreatedTableInOrderWithItsStep](../capture_test.go) |
| A failing step is named in the error | [TestCapture_AFailingStepIsNamed](../capture_test.go) |
| A backup format opens a snapshot that closes | [TestBackupFormatOpensASnapshotThatCloses](../backup_test.go) |
