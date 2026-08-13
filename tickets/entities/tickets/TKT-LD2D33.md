---
id: TKT-LD2D33
type: ticket
title: 'CalDAV: per-user collection state (colour, display name) via PROPPATCH'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Problem

A client's collection-level preferences cannot be stored. Apple Reminders sends
`PROPPATCH` with `calendar-color` on every discovery cycle and gets 501
(go-webdav's `PropPatch` is unimplemented upstream), so a user's colour choice
reverts every time.

Serving a colour read-only (TKT-GFLSFP) makes the OPERATOR's colour work. This
ticket is the other half: letting each USER keep their own.

## Why it needs storage, not just a handler

The value is per `(principal, collection)`. Storing it globally would mean one
user recolouring a shared list for everyone — a silent cross-user write, which
is worse than the current 501.

So this needs a small state service. `internal/caldavalias` is the closest
precedent: its own leaf package, injected at the wiring site, `state.KV`-backed
with an in-process mutex, corrupt-state handling decided deliberately.

Scope it to the properties clients actually send:

- `calendar-color` (Apple, and others)
- `displayname` — a user renaming their copy of a shared list
- possibly `calendar-order` (Apple's sort position)

## Design questions

- **Where does it live?** A sibling of `caldavalias`, or a table inside it? The
alias table is per-resource and this is per-collection, so probably a sibling —
but they share the same persistence and concurrency problem, and one service
handling both may be less machinery than two.
- **Unknown properties.** A PROPPATCH setting something we do not model must be
answered honestly per-property (207 with a per-property status), not blanket 200
— a client told "OK" for a property we discarded will show a value that silently
reverts.
- **go-webdav has no seam.** `caldav.Handler` hardcodes the 501. This needs the
same wrapper approach `withCTag` uses, which is a known-workable but
string-surgery-shaped pattern. Weigh against upstreaming a `PropPatch` hook.
- **Does it survive without a principal?** Under `--read-only` or an
unauthenticated deployment there is no user to key on. Fail closed: keep
answering 501 rather than writing a global value.

## Acceptance criteria

1. A client's colour change persists across polls for THAT principal.
2. A second principal on the same collection is unaffected — pinned by a test.
3. A property we do not model gets an honest per-property failure status, never
a false 200.
4. With no identifiable principal the write is refused, not applied globally.
5. Corrupt stored state fails the way `caldavalias.ErrCorrupt` decided: loud
where it matters, and never silently empty.
