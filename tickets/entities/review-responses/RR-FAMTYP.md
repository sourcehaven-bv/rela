---
id: RR-FAMTYP
type: review-response
title: WorldPrimes took the family type from whichever row arrived first
finding: "storeutil.WorldPrimes recorded a family's entity type from the FIRST candidate seen (`f = &family{typ: c.Type}`) and never revisited it, so a family whose rows disagree on type resolved differently depending on candidate order: [A@'' type=other, A@published type=page] resolved A to '' (rule 1, type unscoped), while the reverse order resolved it to 'published' (rule 2). Unreachable through the write API — all three backends reject a state whose type differs from its family's (StateTypeMismatchError) — but fsstore builds its index from DIRECTORY STRUCTURE ALONE without reading files (scanEntityDirs resolves the type from the dir name), and that load path is deliberately tolerant of hand-edited layouts. A bad merge or a git mv that puts PAGE-1@published.md under entities/others/ produces exactly this shape. The defect is the order-dependence rather than the mis-resolution: everywhere else in this change the answer is deliberately independent of arrival order."
severity: significant
status: addressed
resolution: "WorldPrimes now prefers the DEFAULT row's type when the family has one — the default row is the family's identity row everywhere else in the codebase — so the verdict is stable regardless of candidate order. Documented the single-typed-family assumption and why it is recorded rather than re-derived. Pinned by TestWorldPrimes_MixedTypeFamilyIsOrderIndependent, verified non-vacuous (reverting the preference fails it)."
---

**Finding (code review, TKT-WAV8XP PR-B).**

Not exploitable through the API, but the property that was lost —
order-independence — is one the rest of this change is careful to hold, and
`TestWorldPrimes` already pins it explicitly for the normal case ("candidate
order must not change the answer"). A mixed-type family was the one input where
it did not hold.
