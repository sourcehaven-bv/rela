---
id: RR-5FCJDV
type: review-response
title: Sync write not atomic and non-file config undetectable
finding: OsFS.WriteFile is not atomic and Loader cannot tell whether config is file-backed.
severity: minor
resolution: Sync reads/writes the disk file with temp file + rename and refuses when the file exists only in DB-backed config.
status: addressed
---
