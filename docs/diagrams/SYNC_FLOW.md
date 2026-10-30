# Sync flow

`DB.Sync(models...)` is how a library or module migrates its tables: it creates a table that is missing, adds the
columns a model gained, renames a column the model declares as renamed, and drops a column only when it holds no data.
You run into it every time an app starts a new version: nobody writes migration files. This diagram is the contract
the tests of every consumer walk (each branch has a test).

```mermaid
flowchart TD
    A[Sync models in the order given<br/>parents before children] --> B{Connection runs<br/>transactions?}
    B -->|yes| C[All models in one transaction]
    B -->|no| D[Each model on its own]
    C --> E[CREATE TABLE IF NOT EXISTS]
    D --> E
    E --> F{Connection can list<br/>a table's columns?}
    F -->|no| G[ADD COLUMN for every field<br/>a failure is only logged]
    F -->|yes| H{Listing the columns failed?}
    H -->|yes| X[Sync returns that error<br/>later models are not touched]
    H -->|no| I[Each model field missing from the table]
    I --> J{Model declares an old name<br/>that the table still has?}
    J -->|yes| K[RENAME COLUMN]
    J -->|no| L[ADD COLUMN]
    K --> M{Connection can query data?}
    L --> M
    M -->|no| N[Extra columns are kept<br/>and logged]
    M -->|yes| O[Each table column the model no longer has]
    O --> P{Column holds data?}
    P -->|yes| Q[Kept and logged]
    P -->|no| R[DROP COLUMN]
```
