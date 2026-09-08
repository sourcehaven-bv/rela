---
id: TKT-EMB4RG0
type: ticket
title: 'Time-bound face visibility (embargo / scheduled publication)'
kind: enhancement
priority: low
effort: l
status: backlog
---

A `published` face that EXISTS but must not be readable until a date. rela has
no time dimension in ACL or worlds today, so this is a new capability rather
than a gap in the current one.

## Why it is worth recording now

It separates two properties the current model conflates: a face *existing* and
a face being *readable*. An embargo makes that distinction load-bearing — under
an embargo the published face must be simultaneously present (so an editor can
prepare it) and unreadable (so a reader cannot see it early).

The related disclosure question is now settled in the withholding direction:
`computeFaces` gates each candidate face on `faceReadable` before probing the
store, so a face the caller may not read is not named in `_faces` (QA finding
F-B, fixed in 3531f90b, asserted in the worlds manual). An embargo must not
reopen it — a face under embargo needs the same treatment, absent from `_faces`
rather than listed-but-refused.

## Sketch, not a design

Candidate shapes, none evaluated:

- a `visible_from:` property on a face, evaluated at read time
- an ACL grant carrying a validity window
- a scheduled copy that materializes the face at the embargo moment (fits the
  existing `copies:` machinery and needs no read-path time dependency)

The third is the only one that does not put a clock in the read path, which
matters because `ReadQuery` compiles to a `store.GraphQuery` pushed into SQL —
a time-varying predicate would have to become a SQL predicate in every backend.

## Out of scope for the worlds epic

Recorded so the conformance suite does not grow a scenario asserting behaviour
that does not exist. Raised while planning the ACL/worlds conformance manuals.
