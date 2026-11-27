---
id: TKT-QA4LLQC
type: ticket
title: 'Filter controls: ops allowlist and relative date bounds'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Add an `ops:` allowlist to each list `filter_controls` entry, for example `ops:
[eq, in, gte, lte]`. Part of the smart-lists feature (RES-7SH71TW).

Today the list API accepts `filter[prop][op]` with `eq`, `ne`, `contains`, `in`,
`lt`, `lte`, `gt` and `gte` on any property. A single `eq` or `ne` on a
string-shaped property is already pushed to the store (`planListPushdown` in
`internal/dataentry/listpushdown.go`, via `listPage`), when the rest of the
request qualifies too: no free text, no relation filter and no manual order,
among others. `applyV1Filters` (`internal/dataentry/api_v1.go`) runs the other
operators in Go over loaded rows.

## Scope

- `ops:` on `FilterControl` (`internal/dataentryconfig/config.go`). Validate
each op against the property's type when config loads.
- Relative date bounds for `gte`/`lte` on date properties, resolved at query
time. `$today`, `$today+7d` and `$now` already resolve in filter values
(`resolveFilterVariable`, `internal/dataentry/helpers.go`). Add week bounds
such as `this_week` from the bucket definitions of `group_by.buckets`
(TKT-DOGRL6C, TKT-9F85BEX).
- The SPA filter control offers only the allowed ops.
- Push the remaining declared-control operators (`in`, `contains`, ranges,
non-string types) into the store query instead of the Go post-filter. `eq` and
`ne` are pushed already.

## Open question

What a missing `ops:` means. Defaulting to `eq` keeps today's control behaviour;
decide before changing any default.

## Acceptance criteria

- An op outside the control's allowlist is rejected with a 400 when it comes
from a saved smart list.
- A relative bound resolves against the request's date, not config load time.
- A declared-control filter runs in the store query on postgres.
