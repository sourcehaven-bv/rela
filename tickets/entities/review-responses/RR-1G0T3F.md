---
id: RR-1G0T3F
type: review-response
title: Plan's claim that Redact returns the original pointer when nothing is hidden is preserved is right, but AC 3 tests the wrong thing
finding: 'AC 3 says ''assert Redact still returns the ORIGINAL pointer'' on the NopRedactor path. That is correct and worth pinning, but it only covers len(hidden)==0. The aliasing hazard the plan does not test is the OTHER branch: when redaction DOES apply, Redact does a shallow struct copy (out := *e) and the new Redacted slice would be shared by that copy. Since Redact assigns a freshly-built slice this is safe as written, but Entity.Clone (entity.go:165) must also deep-copy the new field or a clone will alias the original''s slice — the same reason Clone already copies Inaccessible explicitly at entity.go:176-179. The plan lists ''extend Clone'' in Files to modify but has no test for it.'
severity: minor
resolution: 'Addressed. Entity.Clone now deep-copies Redacted via slices.Clone, pinned by TestCloneRedactedIsolation (mirrors the existing TestCloneInaccessibleIsolation). Also added TestRedact_RepeatedCallsDoNotAlias in internal/visibility/redacted_test.go for the branch the original AC 3 missed: Redact shallow-copies the struct, so the copy initially aliases the original''s slice header; building a fresh sorted slice per call is what keeps two redactions of the same entity independent. TestRedact_DoesNotMutateInput additionally guards the store-aliased original.'
status: addressed
---

## Finding

AC 3 pins the `len(hidden) == 0` fast path (original pointer returned). That is
right and worth keeping. But the aliasing risk lives in the other branch, and
neither AC 3 nor any other criterion covers it.

`Redact` does a shallow struct copy on the redacting path:

```go
out := *e
out.Properties = filterProps(e.Properties, hidden)
return &out
```

If `Redacted` is built fresh per call this is safe. But `Entity.Clone`
(`entity.go:165`) copies field-by-field and must learn about the new field too —
exactly as it already does for `Inaccessible`:

```go
if len(e.Inaccessible) > 0 {
    clone.Inaccessible = make([]InaccessibleField, len(e.Inaccessible))
    copy(clone.Inaccessible, e.Inaccessible)
}
```

Omit that and a clone shares the original's backing array. The plan lists
"extend `Clone`" under Files to modify but has no acceptance criterion or test
for it, so nothing would catch the omission.

## Required plan changes

- Add a test that `Clone` produces an independent `Redacted` slice
(mutating the clone's does not affect the original's).
- Add a test that the redacting branch of `Redact` does not alias a
slice across two calls on the same input entity.
