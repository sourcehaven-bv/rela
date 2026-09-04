---
id: RR-KBWJPV
type: review-response
title: Adding a field to entity.Entity affects every store backend's round-trip; plan has no backend conformance test
finding: entity.Entity is the domain type shared by fsstore, memstore and pgstore. The plan asserts the new Redacted field is 'never persisted' but treats that as self-evident rather than as something to enforce. memstore in particular may hold entity pointers or struct copies directly rather than serializing through markdown, so a redacted entity handed back into memstore could retain the marker across a round-trip in a way fsstore would not. The store conformance harness (internal/store/storetest) is the natural place to pin 'a read-out artifact never survives a write round-trip', and the plan does not mention storetest at all.
severity: minor
resolution: 'Addressed, and it FOUND A REAL BUG rather than just pinning an assumption. Added Entity/RedactedNotPersisted to internal/store/storetest/entity.go so it runs against every backend via RunEntityTests. fsstore passed (markdown frontmatter is an explicit field list) but memstore FAILED: it persists via e.Clone(), and Clone had just been taught to deep-copy Redacted for the read-out path, so a redacted entity written back retained the marker. Exactly the backend divergence the finding predicted. Fixed by clearing stored.Redacted = nil in memstore createEntity and updateEntity, at the write boundary where a per-reader artifact should be dropped. Note Inaccessible has the same latent shape in memstore, but harmless there since it genuinely describes stored bytes. Had this test been markdown-only as originally planned, the bug would have shipped.'
status: addressed
---

## Finding

`entity.Entity` is the shared domain type across all three store backends. The
plan states `Redacted` is "never persisted" and lists a negative test ("must NOT
survive a round-trip to markdown storage"), but that test as written targets
**markdown** only.

`memstore` does not serialize through markdown. If it stores entity faces or
struct copies, a redacted entity written back would retain the marker in a way
`fsstore` would not — a backend-dependent behavior difference in a field that is
supposed to be a per-reader artifact.

This is low-likelihood, because the write path should never receive a redacted
entity in the first place (that is exactly what `ReadDeps.WritePrepStore`
prevents, and `TestScriptReads_UpdatePreservesHiddenProperties` — which I
verified exists at `internal/lua/aclreads_test.go:250` — pins it). But
"low-likelihood because another invariant holds" is worth one cheap assertion,
since the whole ticket rests on that separation.

## Required plan changes

- Add the round-trip assertion to `internal/store/storetest` so it runs
against every backend, rather than only against markdown. This is the harness
CLAUDE.md already mandates for store-contract properties.
- Alternatively, if the field is deliberately not a store concern at
all, state that explicitly in the plan and drop the markdown-only test as
misleading.

## Non-finding, recorded for completeness

`entity.Entity` currently has 6 exported fields; the plimsoll `max-fields` cap
is 20. Adding one does not approach the god-object load line and needs no
directive.
