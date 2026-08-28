---
id: TKT-OIRBFH
type: ticket
title: 'Filter language: a token for the requesting principal (@me)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

There is no way to write "the things assigned to me". `internal/filter` has no
principal awareness — no token resolves to the requesting user — so a `where:`
clause cannot reference them. The motivating case is a CalDAV "My tasks"
collection, but the gap is general: ICS feeds, list views and kanbans all take
`where:` clauses and all have the same hole.

## Proposal

A token in the filter language that resolves to the request's principal:

```yaml
    where: ["assignee = @me"]
```

Resolved at match time from `principal.Principal` on the context, not
substituted into the config at load — the same config is evaluated per request
and must yield different results per caller.

## Design questions

- **What does `@me` compare against?** `Principal.User()` is a string identity
from the upstream assertion. If `assignee` holds an entity id (a `person`
entity) rather than that string, the token needs a resolution step — probably a
configured mapping from principal to entity, which is a design decision in
itself and may deserve its own ticket.
- **Unauthenticated / system principals.** `@me` with a zero principal must not
silently match everything. Fail closed: match nothing, and say so in the config
docs.
- **Caching.** Anything that memoizes filter results per collection (the CalDAV
ctag path renders per request today, but that may change) must key on the
principal once results become principal-dependent. Getting this wrong leaks one
user's view to another — worth an explicit test.
- **Does the ACL already cover it?** Where visibility is already gated by
assignee, a plain collection IS "my stuff" without any new token. Check whether
the motivating deployments need this at all before building it.

## Acceptance criteria

1. `@me` resolves to the requesting principal in any `where:` clause.
2. A zero/unknown principal matches NOTHING (fail closed), with a test.
3. Two principals hitting one CalDAV collection see different resources.
4. Works identically in `feeds:`, `views:` and `caldav:` — it is a filter
feature, not a CalDAV one.
