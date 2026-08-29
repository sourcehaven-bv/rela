---
id: RR-EH1K2S
type: review-response
title: No PRAGMA user_version, so a migration ladder could never be retrofitted cleanly
finding: 'schemaSQL is bare CREATE TABLE IF NOT EXISTS with no version marker anywhere in the package. That is not idempotent with respect to CHANGE: it is a silent no-op against an existing table of a different shape, so when a released schema gains a column an old rela.db opens happily and fails at the first query with ''no such column'' -- at runtime, on user data, with nothing pointing at the schema. db_sqlite.go claimed these commands ''become real implementations mirroring db_postgres.go'' when needed, but they could not: a retrofitted ladder cannot tell v1 from v2 because neither stamped a version, so it would have to sniff pragma_table_info to reconstruct which shape it is looking at.'
severity: significant
resolution: 'init now stamps PRAGMA user_version and refuses a database whose version exceeds what the binary knows -- forward-only and fail-loud, the same posture as pgstore.Migrate. An older version with no migration available is also refused rather than opened and mis-read. Exported SchemaVersion() so `rela db status` reports a real number instead of prose. Two tests: a fresh database records version 1, and one stamped 99 is refused with an error naming the mismatch.'
status: addressed
---
