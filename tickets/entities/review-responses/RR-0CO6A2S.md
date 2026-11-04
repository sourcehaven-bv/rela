---
id: RR-0CO6A2S
type: review-response
title: Decision mode derived in the composer, not owned by the app
finding: 'Decision mode keyed on current being a button type: the refusal path (refused, useSessionDecisions.ts:561) opens a type with no form decision, so Send has no expect_generation and the stale watch never fires; a transient default selection could also land in decision mode.'
severity: significant
resolution: 'Plan 2 and 3: every form decision records a compose view (given empty for a plain one) and the composer shows a button type only while compose names it; the refusal path records a form decision at the click generation (AC 4).'
status: addressed
---
