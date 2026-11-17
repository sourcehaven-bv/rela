---
id: TKT-4Z2BIRG
type: ticket
title: Push date comparisons against today() and now() down to the store
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

List conditions and query scopes that compare a date or datetime with `today()`
or `now()` run on the Go path (TKT-08ZI5XN). Lower them to the store exactly.

Needed first:
- Canonical `YYYY-MM-DD` storage for `date` on the write path, plus a data migration. Today fs decodes an unquoted date as time.Time, `ParseDateValue` accepts RFC 3339 with offsets, and legacy rows hold values such as `TBD`.
- Shape-checked range operators in every backend, so a malformed value matches no row, as in Go.
- A C-collated index with EXPLAIN tests on postgres and sqlite.

Then lower guarded leaves (`entity.due ~= nil and entity.due < today()`) using
the read time on ctx (`internal/readtime`) as a request constant. Day-bound
table from the TKT-08ZI5XN design review (RR-7LRJ1TO, RR-OCS1CUV).
