---
id: RR-IIRWWXV
type: review-response
title: Error line numbers refer to re-encoded YAML
finding: After an in-memory migration a parse or validation error reports lines of the migrated text.
severity: minor
resolution: Migrated.Explain appends a note that line numbers refer to the migrated form; used by the schema loader and loadConfig.
status: addressed
---
