---
id: TKT-D3BQJ5J
type: ticket
title: 'pgstore: a binary older than the schema writes stale session facts'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

pgstore runs migrations forward but does not refuse a database schema that is
newer than the binary. sqlitedb does refuse it (`sqlitedb/migrate.go`).

During a rolling deploy or a rollback, a binary from before migration 0040 can
run against a migrated database. That binary moves sessions without writing
`session_previous`, and before 0039 also without writing `session_cause`. The
columns then keep the values of an earlier move. The web app shows a stale
previous state as if it were current, and the inbox can point at the wrong
waiting message.

Found in the code review of TKT-56TPOTE (finding M4). That ticket did not decide
it, because it is a deployment policy for every pg migration, not only the two
channel columns.

## Options

1. pgstore refuses to start on a schema newer than the binary, as sqlitedb does.
This blocks a rollback across a migration until the database is restored.
2. Store the generation with each fact of the last move (for example
`session_previous_gen`) and read the fact as empty when it does not equal
`session_gen`. This tolerates mixed versions for these columns only.
3. Document that `cause` and `previous` are unreliable after a downgrade.

## Acceptance criteria

- A decision on the policy for a pg schema newer than the binary.
- The chosen option implemented with a test, or the limitation documented.
