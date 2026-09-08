---
id: RR-IZ9XNH
type: review-response
title: ResolveQueryIdentity is documented as the single derivation point but has no production caller
finding: predicatefns.ResolveQueryIdentity's doc claims it is where every transport derives the identity and that resolver errors must not degrade to the raw principal. Production uses appbuild.queryIdentityFor, which infers the resolution outcome from RawUser != User and inherits resolvePrincipalEntity's fail-open on a backend error (raw email compared against entity ids — matches nothing, narrowing).
severity: minor
resolution: 'Kept, since the boundary stamp in TKT-ZQV9O5 is its intended caller: ResolveQueryIdentity''s doc now says no boundary calls it yet and names that ticket; queryIdentityFor''s doc records the backend-error degradation (matches nothing, never widens) so the next reader does not assume the stronger contract.'
status: addressed
---
