---
id: RR-MHD4DH
type: review-response
title: A new auto.when rule never closes channels of entities that already match
finding: '§3 step 2 baselines every new rule silently, auto rules included, and D2 fires on edge only. An operator who adds `auto: {when: status == ''closed''}` therefore leaves the rooms of every entity that is already closed open forever. A transition is idempotent and posts nothing, so auto rules should act on their first evaluation (CAS-safe) rather than baseline silently. State this in AC14.'
severity: significant
resolution: 'Per Jeroen: new triggers and auto rules both start from now. Documented in docs/realtime.md and pinned in AC14.'
status: addressed
---
