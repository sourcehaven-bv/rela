---
id: RR-74XCVE
type: review-response
title: lessSource lacks a Claim tiebreak; widening subjectVerifier naively defeats the seam's purpose
finding: Two ordering/layering gaps. (1) lessSource (acl/source.go:152-163) sorts on priority, Group, Ancestor, Relation. A new Source.Claim field not added to that chain makes two asserted attributions differing only by Claim compare equal, so PrimarySource and mergeRoutes (aclmap/whocan.go:98-112) produce non-deterministic output. The plan mentions adding a lessRoute tiebreak for aclmap but not lessSource for acl; both need it. (2) The plan says to 'widen the subjectVerifier seam (router.go:362)'. That interface is deliberately one method returning (string, error) so dataentry does not import jwtauth, per the consumer-side-interface rule in CLAUDE.md. If widening means returning a jwtauth.AssertionClaims, dataentry now imports jwtauth and the seam's entire purpose is gone — precisely the 'leak implementation types via return values' failure CLAUDE.md calls out. The plan gestures at the adapter pattern but never states the resolution.
severity: significant
resolution: 'Both parts accepted. (1) Confirmed lessSource (acl/source.go:152-163) sorts on priority/Group/Ancestor/Relation only; Claim is added to that tiebreak chain AND to lessRoute in aclmap, with a test that two attributions differing only by claim sort deterministically. Without it PrimarySource and mergeRoutes emit non-deterministic order, which would surface as a flaky golden-artifact test rather than an obvious bug. (2) Seam shape decided explicitly: option (a) — dataentry declares its own small claims struct, and the cmd/rela-server adapter translates from jwtauth.AssertionClaims, exactly as webhookVerifierAdapter already does (main.go:244-252). dataentry does NOT import jwtauth, so the consumer-side-interface rule holds and this composes with RR-CHW2AA keeping jwtauth a leaf. Returning jwtauth.AssertionClaims through the seam is explicitly rejected — that is the ''leak parsing types via return values'' failure CLAUDE.md names.'
status: addressed
---

## Finding

**1. `lessSource` has no `Claim` tiebreak.**

`acl/source.go:152-163` sorts on `(priority, Group, Ancestor, Relation)`. A new
`Source.Claim` field absent from that chain makes two asserted attributions
differing only by claim compare **equal**, so `PrimarySource` and `mergeRoutes`
(`aclmap/whocan.go:98-112`) emit non-deterministic order. The plan calls out the
`lessRoute` tiebreak for `aclmap` but omits `lessSource` for `acl`. Both need
it.

**2. Widening `subjectVerifier` naively defeats its purpose.**

The plan says "widen the `subjectVerifier` seam (`router.go:362`)". That
interface is deliberately one method returning `(string, error)` so `dataentry`
does not import `jwtauth` — the consumer-side-interface rule in CLAUDE.md.

If widening means returning a `jwtauth.AssertionClaims`, `dataentry` imports
`jwtauth` and the seam's whole reason for existing is gone. This is exactly the
"don't leak storage or parsing types via return values" failure CLAUDE.md names.

## Resolution

- Add `Claim` to the `lessSource` tiebreak chain and to `lessRoute`; test that
two attributions differing only by claim sort deterministically.
- Pick one seam shape explicitly: (a) `dataentry` declares its own small claims
struct and the `cmd/rela-server` adapter translates, or (b) the interface
returns the individual values. Either is fine; leaving it unstated is not.
