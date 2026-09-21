---
id: RR-HP70BX
type: review-response
title: 'Minor review findings: fragile length constant, missing live hash, unused error type, inaccurate test claim'
finding: 'Batch of minor findings from the code review. (a) slugify derived its truncation budget from the literal string "20260919143022-.yaml" as a length proxy, which breaks silently if stampLayout changes. (b) Describe() for StatusUnbaselined omitted the live hash that every other branch reports, in the case where an operator most needs it. (c) NewMigrationFileName does not validate that stamp is stampLayout-formatted (latent only - the sole caller formats correctly). (d) StateFromFutureError is returned and wrapped but nothing branches on it with errors.As. (e) My own earlier claim that migrateface_test.go was left untouched was wrong: it changed by 15 filename swaps; no assertion logic changed, which is the substance, but the claim as stated was inaccurate.'
severity: minor
resolution: 'Fixed (a) budget now derives from len(stampLayout); (b) Describe reports the live shape; (c) doc states stamp must be stampLayout-formatted. Accepted as-is: (d) StateFromFutureError''s value is in its error text today, and refusing is refusing - it earns its keep if a caller later wants a distinct CLI message. Corrected (e) in the implementation checklist.'
status: addressed
---

## Findings and disposition

**(a) `slugify`'s truncation budget was a literal length proxy** — `name.go`.
The arithmetic was *correct* (the reviewer verified it empirically, as did my
own fuzzing), but `len("20260919143022-.yaml")` as a stand-in for the stamp
length breaks silently if `stampLayout` changes. **Fixed:** derives from
`len(stampLayout) + len("-") + len(".yaml")`.

**(b) `Describe()` for `StatusUnbaselined` omitted the live hash** — `gate.go`.
Every other branch reports it, and this is the case where an operator most needs
it: to compare against a migration file's `to_projection` by hand before
deciding how to resolve. **Fixed:** now reports `short(v.LiveHash)`.

**(c) `NewMigrationFileName` does not validate `stamp`** — `name.go`. A
non-`stampLayout` stamp yields an error from `ParseMigrationName` rather than a
bad name, and the sole caller formats correctly, so this is latent only.
**Fixed:** documented that `stamp` must be `stampLayout`-formatted.

**(d) `StateFromFutureError` is defined and returned but nothing uses
`errors.As`** — `state.go`. The distinction it encodes (bootstrap vs. refuse) is
load-bearing, but in practice a future-version state and a corrupt one both
surface as a wrapped error at the same call sites. **Accepted as-is:** refusing
is refusing, and the value today is in the error *text* — `filemigstate`'s test
asserts the "upgrade rela" wording reaches the operator. The type earns its keep
if a caller later wants a distinct CLI message.

**(e) My "migrateface_test.go was left untouched" claim was wrong.** The
reviewer caught this. The file changed by 15 filename swaps (`"0001-faces.yaml"`
→ `testName("faces")`). No assertion logic changed — which is the substance of
the claim, and the BUG-TMGWIN guard genuinely survives untouched in `file.go` —
but "untouched" was inaccurate as stated. **Corrected** in IMPL-J5RMKW.

## Deferred

The reviewer's leverage suggestion to thread `MigrationName` through
`AppliedEntry.Name`, `File.Name` and `Resolve` is sound — it would have made the
legacy-name drop (RR-QQLKDE) a visible conversion point rather than a `continue`
inside a loop. Not done here: it touches every call site in the package and the
defect it would have surfaced is now fixed and pinned. Worth a follow-up ticket
if the string-typed boundary causes trouble again.
