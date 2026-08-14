---
id: TKT-KUAV99
type: ticket
title: 'Make field-level redaction structural: a distinct type for render-safe entities'
kind: refactor
priority: medium
effort: l
status: backlog
---

## Problem

`docs/acl-security.md` asserts that `visible:` redaction is applied "on
**every** HTTP read shape". That is a guarantee in prose and a **convention** in
code: each read path calls a redaction helper itself, and nothing stops a new
one from skipping it.

Two paths did skip it, independently, and neither was caught by review:

- **CalDAV** (PR #1308) — `renderObject` read `e.GetString(...)` off the raw
entity. Found by security review.
- **The ICS feed** (BUG-E9DYW5, PR #1314) — `mapEntity` did the same. Found only
because CalDAV inherited the pattern from it.

Both compiled, passed review, and served hidden property values.

## Root cause

There is no type-level distinction between "raw entity from the store" and
"entity safe to render outward". Both are `*entity.Entity`, so:

```go
e, _, _ := src.getEntity(ctx, typ, id)   // row-gated, NOT field-redacted
ev.Summary = e.GetString("title")        // compiles; may leak
```

This is precisely the bug class `visibleReader` was built to close, one level
down. Its own doc comment states the principle:

> the read gate already exists but is applied by *convention* — a handler can
> reach `a.store.GetEntity` directly and forget to gate. That "gate by
> convention" is the read-ACL bug class (TKT-N26KLB, #1010). A type that holds
> the store privately and exposes only gated reads makes the gating
> **structural**.

`visibleReader` made ROW gating structural and returns `*entity.Entity` — so
FIELD redaction remained conventional. The fix is the same move applied to
fields.

## Current state: four mechanisms, one contract

The same "omit hidden properties" rule is implemented four times over:

| Site | Shape |
|---|---|
| `affordanceService.stripHiddenProperties` | mutates a `v1.Entity` in place |
| `affordanceService.copyVisibleProperties` | returns a fresh property map |
| `redactEntityFields` (CalDAV) | returns a copied `*entity.Entity` |
| `redactFeedEntity` (ICS feed) | returns a copied `*entity.Entity` |

The last two are near-duplicates added while fixing the two leaks — evidence the
missing abstraction is being re-derived per site rather than reused.

## Proposal (shape to be settled in planning)

A `visibility.RedactedEntity` (or similar) that:

- **cannot be constructed** except by passing a raw entity through the redactor
- exposes only read accessors (`GetString`, `Content`, …), never the raw
`Properties` map
- is what `visibleReader` returns from its read-out methods, so a consumer
taking the gated reader gets field redaction by construction

Then a render path that wants a property has no way to reach an unredacted one,
and the four mechanisms above collapse to one.

## Open questions for planning

- **Write-prep reads must NOT be redacted.** `entitymanager` diffing needs the
raw entity — a redacted read-modify-write would clobber hidden fields, which is
exactly the erasure `PatchEntity` exists to prevent. The type must not reach
that path, or must be explicitly convertible with a named, greppable escape
hatch.
- **The body (`Content`) has no `visible:` vocabulary.** CalDAV clears it when a
property named `body` is hidden, which is a heuristic. Decide whether the type
models body visibility properly or preserves that heuristic.
- **Filter/sort inputs stay raw.** The ICS feed evaluates `where:` against the
unredacted entity on purpose — redacting first would make feed *membership* vary
per principal. Any type must leave that path a raw entity.
- **Scope.** `visibleReader`'s own doc notes some ungated reads were deliberately
not migrated (BUG-ZM7SBI). This ticket should not silently widen that.

## Acceptance criteria

1. A render path cannot obtain an unredacted property value without a named,
greppable escape hatch.
2. The four existing redaction mechanisms collapse to one implementation.
3. Write-prep reads still receive raw entities;
`TestPatchEntity_PreservesUnnamedProperties` and
`TestScriptReads_UpdatePreservesHiddenProperties` still pass.
4. Feed membership is still decided on unredacted values
(`TestDeclarativeFeed_RedactionDoesNotChangeMembership` still passes).
5. A new read path that forgets redaction fails to COMPILE, and a test
demonstrates that (e.g. a compile-fail fixture or a lint rule).
