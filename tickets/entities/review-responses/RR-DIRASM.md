---
id: RR-DIRASM
type: review-response
title: relationChoices asserts a per-group direction invariant the server does not guarantee
finding: 'relationChoices takes edges[0].direction as representative of the whole group, with a comment asserting every edge in a group shares a direction. That is false for a self-loop (verified: the same edge appears under both keys) and for a symmetric self-inverse relation, where the handler appends outgoing then incoming entries into one slice.'
severity: minor
resolution: 'Superseded in practice: self-loops (the mixed-direction case) are now dropped before grouping matters, and the symmetric case is handled by the corrected direction test. The comment''s claim is narrowed accordingly.'
status: addressed
---

## Suggested resolution

Derive direction defensively per edge, or cite where the invariant is enforced. Related to RR-SLFLP and RR-SYMIN.
