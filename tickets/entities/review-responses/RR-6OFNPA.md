---
id: RR-6OFNPA
type: review-response
title: The 'served, not injected' exception is asserted on ownership, not on the real (security) reason
finding: 'The plan honestly flags that internal/dataentry/CLAUDE.md:158-161 forbids server-side HTML rewriting
  for apps, and argues the SPA shell differs because ''we own the SPA shell; we do not own an app''s HTML''.
  The conclusion is right but the stated rationale is weaker than the real one. CLAUDE.md:146-157 shows
  the app CSP is ''the whole boundary'' — path-scoped script-src, connect-src ''none''. Rewriting an app''s
  index is forbidden because injected script would need a CSP allowance, which would puncture the only
  thing standing between a sandboxed app and the API. The SPA shell has no CSP at all (the plan notes
  this itself). So the correct argument is: rewriting an app index would weaken a security boundary; rewriting
  the SPA shell crosses no boundary because none exists there. That is far more durable, and crucially
  it tells a future reader the exact condition under which the exception becomes INVALID — namely if a
  CSP is ever added to the SPA route.'
severity: significant
status: addressed
resolution: internal/dataentry/CLAUDE.md now states the SECURITY-boundary argument (an app CSP is the
  whole boundary confining an untrusted app; the SPA shell has none, so the rewrite crosses no boundary)
  with an explicit TRIP-WIRE that the exception lapses if a CSP is ever added to the SPA route. Also mirrored
  in the router.go comment at the registration site.
---

## Recommended resolution

Write the security-boundary argument (not the ownership argument) into
`internal/dataentry/CLAUDE.md`, with the trip-wire stated explicitly:

> This exception holds only while the SPA route has no CSP. If a CSP is ever
> applied to `/`, this injection must be revisited.

`TKT-3DBK6I` already gestures at this in its Security section ("If a CSP is ever
applied to the SPA route it must permit these paths") — promote it from a
passing security note to the stated boundary condition of the exception.
