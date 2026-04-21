---
id: RR-6HR8S
type: review-response
title: 'Race condition: prefix list could change between schema fetch and entity create'
finding: Frontend loads schema once on app mount; if the user leaves the app open and the metamodel changes on disk (adding/removing a prefix), the prefix <select> shows stale options. Submitting the stale value would hit server-side validation (good). The plan should note that stale-prefix errors are acceptable with a clean 422 — no need for defensive fetches. Minor because SSE reloads already cover most of this; mention in the plan so the behavior is deliberate.
severity: minor
resolution: 'Plan''s Security section now documents stale-schema behavior: ''If the metamodel changes between schema fetch and create, the user sees a clean 422 on submit. No defensive fetch needed.'''
status: addressed
---
