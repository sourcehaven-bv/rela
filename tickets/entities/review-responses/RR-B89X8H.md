---
id: RR-B89X8H
type: review-response
title: admin.get_entity masked infrastructure errors as "does not exist"
finding: 'elevatedGetEntity pushed lua.LNil on ANY error from er.GetEntity, so a store outage was indistinguishable from a genuine miss. Verified: a reader returning "connection refused: database is down" gave the script nil and no error. Two compounding problems. (1) The docs added in the same commit promise "admin.get_entity returns nil only when the entity genuinely does not exist" — false under any store error, and the whole point of contrasting it with the oracle-free gated nil is that the elevated nil is supposed to be informative. (2) The documented use case is a correctness invariant: the guide''s own example is a cross-entity uniqueness check, so under a transient outage the check reads nil, concludes "no duplicate", and silently violates the invariant the elevated read exists to enforce — precisely when the system is already unhealthy. The sibling list_entities/get_relations already raised on iteration errors, making get_entity the inconsistent one.'
severity: significant
resolution: get_entity now returns nil ONLY for store.ErrNotFound and raises on every other error, making the three read methods consistent. Corrected the false claim in GUIDE-lua-scripting.md and regenerated docs. Added TestElevatedRead_StoreErrorIsNotMaskedAsMissing plus TestElevatedRead_MissStillReturnsNil (so the fix cannot swing the other way and break existence checks); mutation-verified.
status: addressed
---

Raised by the cranky-code-reviewer against commit `31813351`.

The most consequential of the three findings: it made my own documentation
false, and the failure mode is silent corruption of a correctness invariant
rather than a visible error. The gated `rela.get_entity` mirrors this
nil-on-error shape, but there it is defensible — the gated nil is *deliberately*
ambiguous to stay oracle-free. The elevated nil carries the opposite contract,
so the same code shape has the opposite meaning.
