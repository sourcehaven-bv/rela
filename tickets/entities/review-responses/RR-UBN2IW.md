---
id: RR-UBN2IW
type: review-response
title: Confirm dialog re-finds the view by key; a preset key can point to another preset after a reload
finding: After the dialog, onDecide posts find(key) (frontend/src/components/entity/channels/useSessionDecisions.ts:309-314). A reload that reorders entries at the same generation leaves the generation check passing, while preset:<type>:<entry> now names a different label and props than the user confirmed. Post the confirmed view's props, or treat a changed label or props as stale. Add the case to useSessionDecisions.test.ts.
severity: minor
resolution: After the confirm dialog, onDecide posts the view the user confirmed and does not look it up again by key; expect_generation covers staleness. A useSessionDecisions test case pins this.
status: addressed
---
