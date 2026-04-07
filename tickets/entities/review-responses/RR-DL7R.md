---
id: RR-DL7R
type: review-response
title: 'L1: per-instance session token defence in depth'
finding: Reviewer suggested a per-instance random session token in a custom header as a stronger CSRF defence on top of the Origin allowlist. Custom headers can't be forged by `<img>`/`<form>` and require a preflight that the Origin gate already blocks.
severity: nit
reason: Already in the original ticket scope as 'follow-up' (out-of-scope section). The Origin allowlist alone is sufficient for the documented threat model. Token plumbing through the Vue SPA is non-trivial and would benefit from its own ticket with a clean API surface.
status: deferred
---
