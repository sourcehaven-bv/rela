---
id: BUG-TY2XQC
type: bug
title: Two pgstore migrations share version prefix 0003 — one can be skipped on an already-migrated DB
priority: medium
why1: loadMigrations parses the integer filename prefix as the version; 0003_sync.sql and 0003_attachments_per_file_pk.sql both parse to version 3.
why2: Migrate applies migrations where version > current schema_version and sorts by version; on equal versions the sort order between the two 0003 files is not guaranteed, and once schema_version reaches 3 the second-sorted 0003 is skipped (version <= current).
why3: The migration filenames were chosen independently (sync vs attachments work landed close together) with no check enforcing unique version prefixes.
status: backlog
---
