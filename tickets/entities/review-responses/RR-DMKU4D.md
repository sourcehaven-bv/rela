---
id: RR-DMKU4D
type: review-response
title: Phase 3 gated behind phase 2 partial matches
finding: Phase 2 can return a body-only paragraph candidate (Jaccard ~0.85) for a heading+long body quote, so phase 3 never runs and the comment re-anchors without the heading.
severity: critical
resolution: 'Plan revised: phase 3 runs alongside phase 2 when phase 1 finds nothing; candidates compete on score. Test added for heading + long body where a body-only phase-2 candidate exists.'
status: addressed
---
