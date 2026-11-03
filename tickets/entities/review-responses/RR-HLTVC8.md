---
id: RR-HLTVC8
type: review-response
title: The db import-fs table test will fail on the new table
finding: TestDBImportFS_SQLiteTablesAreAccounted (internal/cli/db_importfs_sqlite_test.go) fails for any new table it does not list. Add channel_reads with the TKT-DD8B6I "not copied yet" reason, and add the file to "Files to modify".
severity: nit
resolution: channel_reads added to TestDBImportFS_SQLiteTablesAreAccounted with the TKT-DD8B6I reason; file listed in Files to modify.
status: addressed
---
