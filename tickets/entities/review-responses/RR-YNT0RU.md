---
id: RR-YNT0RU
type: review-response
title: Stale prefill values outlive the compose decision and get posted
finding: 'The prefill writes into `drafts[type]` and keeps every other draft value (AC 8). The plan''s risk note accepts this because "the next compose click writes the named fields again", but that covers only the named fields. Cases: (1) Compose button A sets `{decision: revise, topic: X}` and button B of the same type sets `{decision: reject}`. After A then B, the draft still has topic X, which B never set. (2) After a cancelled compose (the person switches type), the prefilled `decision` stays in the type''s draft (RlChannelComposer.vue:251-252, 272-278). A later manual pick of that type posts it without the person choosing it, and the required check no longer catches it. (3) A second click overwrites fields the person already edited. Fix: record which names the last prefill wrote. On a new prefill, or when the compose decision ends, clear the names the person did not touch. Add story and useSessionDecisions cases for A-then-B and for cancel-then-manual-select.'
severity: significant
resolution: ADDRESSED Given values never enter drafts, so nothing outlives compose mode. A-then-B, cancel-then-manual-select and a second click are covered in stories and useSessionDecisions cases (AC 8, AC 10).
status: addressed
---
