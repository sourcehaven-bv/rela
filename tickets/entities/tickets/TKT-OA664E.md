---
id: TKT-OA664E
type: ticket
title: Copy-on-write editing from fallback faces
kind: enhancement
priority: medium
effort: m
status: backlog
---

Design doc §9.3. Worlds declare an `edits:` target (declared, never inferred
from the chain or from grants). The edit affordance on a fallback-face view maps
to: create the target state seeded from the exact viewed prime (per provenance),
apply the delta. Seed copy is same-entity → elevated (hidden-field preservation,
the never-redact-a-write-prep rule). Offered through the `_actions` affordance
machinery, re-authorized server-side. The fallback chain then flips the prime
for all readers of that world — inherent and correct for a pending-work world.
Auto-fork vs explicit "start revision" is UX policy on the same primitive.
