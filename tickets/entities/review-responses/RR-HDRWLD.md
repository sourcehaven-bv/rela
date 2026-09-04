---
id: RR-HDRWLD
type: review-response
title: ListEntityHeaders was the seventh refusal-protected entry point with no world conformance coverage
finding: "Enumerating the pgstore entry points currently guarded by PR-B's temporary world refusal gives SEVEN (ListEntities, ListEntityHeaders, ListEntitiesPage, CountEntities, GraphQuery, GraphCount, MatchingIDs). RunWorldTests covered six of them. ListEntityHeaders had NO world coverage on any backend: the dedicated header conformance suite (storetest/header.go) passes no World in any of its cases, and RunWorldTests never touched the header path. The gap was invisible because ListEntityHeaders is an OPTIONAL capability (store.HeaderReader) — fsstore does NOT implement it and is served by the generic store.ListEntityHeaders fallback over ListEntities, which is world-correct for free, so the path looked exercised. memstore and pgstore implement it NATIVELY, and a native implementation is free to serve the default world while the list beside it serves the requested one. pgstore's PR-B refusal was the only thing standing in for that coverage — and PR-C's job is to delete the refusal."
severity: significant
status: addressed
resolution: "Added conformance case Worlds/HeaderPathHonorsTheWorld, routed through the package-level store.ListEntityHeaders helper so BOTH arms are exercised — fsstore's generic fallback and memstore's native implementation. Same `otherwise: exclude` shape as GraphQueryHonorsTheWorld: published face only, excluded entity absent, at-most-one-prime asserted. Verified non-vacuous against the NATIVE path specifically (neutering memstore's ListEntityHeaders to pass a zero WorldScope fails the case) — verifying only via fsstore's fallback would have proved nothing, since that arm cannot fail this way. Landed in PR-B rather than PR-C because it closes a gap in PR-B's own conformance suite; the effect is that all seven pgstore entry points have coverage waiting BEFORE PR-C flips Capabilities.Worlds, rather than acquiring it afterwards."
---

**Finding (PR-C readiness check, TKT-WAV8XP PR-B).**

Prompted by the architect's PR-C sequencing instruction: *every site PR-C worlds
is a site that was previously protected by the refusal, so the conformance suite
must cover each one BEFORE the flag flips, not after.* Taking that literally —
enumerating the guarded entry points and diffing them against `RunWorldTests` —
surfaced the one path where "looks covered" and "is covered" diverged.

**Same class as RR-GQWRLD.** In both cases a path appeared safe because a
*different* implementation of it happens to be correct: there, `pgstore` was
safe only via its refusal while fs/mem failed open; here, the header path looks
exercised only because the backend that lacks the capability inherits a correct
fallback. The general lesson for the rest of this arc: **an optional store
capability needs its own world coverage, because the backend that does NOT
implement it cannot demonstrate the bug.**
