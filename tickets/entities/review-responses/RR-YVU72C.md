---
id: RR-YVU72C
type: review-response
title: SessionStep has no From, so the first move records from=""
finding: The store writes the notice and the audit record inside Append, but SessionStep carries only Expect (stored state, "" while unset) and To. Under the "unset until first transition" decision the store cannot know the initial state's name, so either the notice says from "detected" (service-resolved, via the SessionNotice from-parameter no backend can supply) while the audit says From "", or both say "". AC 6 expects props from/to and an audit From. Add SessionStep.From (the state name the service resolved, initial for an unset session), validate it in PrepareSessionStep, and have SessionNotice and SessionAudit both take it from the step. Assert in testSessionStep that the notice and the audit carry the same non-empty From for an unset session.
severity: significant
resolution: 'Plan updated: the store decides the move (From, To) under its lock, resolving an unset session''s From to initial; notice and audit take From from that move; testSessionStep asserts they match and are non-empty.'
status: addressed
---
