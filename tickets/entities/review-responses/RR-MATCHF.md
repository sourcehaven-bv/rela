---
id: RR-MATCHF
type: review-response
title: PaginateWorldPrimes filters rows BEFORE resolution, so a per-row match would corrupt absence detection
finding: "storeutil.PaginateWorldPrimes applies its `match` callback before appending a row to the family buffer, so only rows that survive `match` ever reach WorldPrimes. But WorldPrimes' own contract states candidates must be WHOLE families, because resolution decides on ABSENCE: with the full family [A, A@published] a published-world resolves A to A@published, whereas feeding it only [A] fires the fallback instead. The same shape exists in fsstore.keepPrimes and memstore.worldKeep, which resolve over the already-matched set. NOT currently exploitable — MatchEntityQuery filters only on Type and IDs, both constant across a family — but the invariant that makes it safe lives in a DIFFERENT function and was stated at neither site. Any future per-row predicate pushed into match (a property filter, an updated-at bound, a pointer fast path) silently breaks world resolution in the serve-the-wrong-face direction."
severity: significant
status: addressed
resolution: "Documented the requirement loudly at PaginateWorldPrimes: match MUST be family-constant, with the concrete failure (filtering away the published row makes the chain look unsatisfied, so the fallback serves the default face in a world that meant to replace or exclude it) and the correct alternative (resolve the family first, filter the primes afterwards). Left as documentation rather than a structural guard because the invariant cannot be expressed in the callback's type; flagged for the architect as a candidate for PR-D's structural-guard pass if a per-row predicate is ever wanted."
---

**Finding (code review, TKT-WAV8XP PR-B).**

A latent landmine rather than a live bug. Recorded because the surrounding code
is documented specifically against this class of mistake, and this was the one
place the reasoning was left implicit.
