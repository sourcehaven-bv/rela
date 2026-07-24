---
id: RR-S14ZKC
type: review-response
title: Principal accessors handed out the roles backing array by value
finding: Principal is a value type, so a plain struct copy shares the roles slice backing array even though Verified() clones on the way in and Roles() clones on the way out. acl.Request.Principal() (request.go:151) returned r.principal by value, and principal.From(ctx) returned the ctx-shared value — so a caller could reslice onto the shared array and mutate a role another holder was about to authorize against. Not currently exploitable (no caller does this), but the type's entire premise is that the compiler enforces the trust boundary, and this was the one place it stopped doing so.
severity: minor
resolution: 'Added Principal.Clone() (deep copy of the roles slice) and used it at both by-value hand-out sites: acl.Request.Principal() and principal.From(ctx). From(ctx) matters most — the ctx value is shared by every reader for the life of a request, so aliasing there is the widest exposure. TestPrincipal_AccessorsDoNotShareBackingArray asserts both: that Clone''s array is independent, and that two From(ctx) results on the same context do not alias each other.'
status: addressed
---

## Finding

`Principal` is a value type. `Verified` clones on the way in and `Roles()`
clones on the way out, but a plain **struct copy** still shares the `roles`
backing array.

Two sites handed one out:

- `acl.Request.Principal()` (`request.go:151`) — returned `r.principal` by
value while the Request was still authorizing against it.
- `principal.From(ctx)` — returned the ctx-shared value to every reader.

A caller could reslice onto the shared array and mutate a role another holder
was about to authorize against.

Not currently exploitable — no caller does this. But the type's entire premise
is that the compiler enforces the trust boundary, and this was the one place it
stopped doing so.

## Resolution

Added `Principal.Clone()` (deep-copies the roles slice) and applied it at both
hand-out sites. `From(ctx)` is the more important of the two: the ctx value is
shared by every reader for the life of a request, so aliasing there has the
widest blast radius.

`TestPrincipal_AccessorsDoNotShareBackingArray` asserts both properties — that a
`Clone`'s array is independent, and that two `From(ctx)` results on the same
context do not alias each other.
