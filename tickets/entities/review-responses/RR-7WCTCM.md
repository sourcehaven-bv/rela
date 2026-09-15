---
id: RR-7WCTCM
type: review-response
title: An unidentified caller on an identity scope gets a 500 blaming entity loading
finding: A scope reading current_user, evaluated for a caller the deployment cannot identify, surfaces ErrNoCurrentUser wrapped in errListLoad — so writeListPipelineError answers 500 'Loading entities failed'. The direction is right (refusing beats serving the unfiltered set, which for a 'assigned to me' list would mean everybody's rows), but the status and message name the wrong thing, exactly as the bad-scope-name case did before it was classified.
severity: minor
reason: 'Deferred, not dismissed. The fix needs a sentinel to cross the dataentry/condition-engine seam, and neither side may name the other''s errors: dataentry cannot import predicatefns, and appbuild cannot import dataentry (the cycle the whole structural-adapter design exists to avoid). So it needs a seam decision — most likely an error the adapter translates at the composition root — rather than a local edit. I started it, found it widening, and stopped rather than reshaping a seam late in review. The failure is loud and correct in direction; only its label is wrong.'
status: deferred
---

Worth doing alongside [[TKT-LYLO6P]], which touches the same seam for the Lua
and MCP opt-in and will have to answer the same question about how an identity
requirement is reported to a caller that has none.

Note the next-action engine's answer does not transfer: it SKIPS a per-user
source for an unidentified caller, because a missing suggestion is merely less
helpful. Skipping a scope would serve the unfiltered set, so the error has to
stay an error.
