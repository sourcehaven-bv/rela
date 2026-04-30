---
id: RR-V4ZZC
type: review-response
title: Double-call default of silently resolving false masks bugs
finding: 'Plan: ''confirm() while one is open → second resolves false''. This is a quiet failure mode. Better: return the in-flight promise so both callers see the same answer, or throw in dev (import.meta.env.DEV) and resolve false in prod. Pick one and document. The chosen ''silent false'' option is the worst because it hides bugs.'
severity: significant
resolution: 'Switched to: second concurrent call returns the in-flight promise so both callers observe the same user decision. Documented in JSDoc and tested.'
status: addressed
---
