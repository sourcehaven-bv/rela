---
id: RR-PDKNZG
type: review-response
title: Identity was derived and re-stamped per candidate, not once per request
finding: WithQueryIdentity's godoc and the ticket say the identity is stamped once per request by the wiring site, never derived per row. As built, appbuild's nextActionMatcher.Match derived it (QueryIdentityFrom -> queryIdentityFor -> context.WithValue) on every call, and nextaction.applyCondition calls Match once per candidate. Cheap only because the derivation skips the ACL resolver; the planned convergence on ResolveQueryIdentity would have made it one store lookup per row.
severity: significant
resolution: The NextActionMatcherFunc seam now also returns a per-request scope binder (dataentry.NextActionRequestScope). handleV1NextActionGet applies it to the request ctx once before eng.Resolve; appbuild's Match and Prefilters only read predicatefns.QueryIdentityFrom and never derive. TestNextActionMatchers_PrefiltersAndMatchShareTheIdentity asserts an unscoped ctx with a principal yields no identity.
status: addressed
---
