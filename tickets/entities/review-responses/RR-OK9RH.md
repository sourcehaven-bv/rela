---
id: RR-OK9RH
type: review-response
title: checkboxes skipped test masks product-vs-test-infra uncertainty
finding: Skip reason says 'The behaviour is a real regression risk'. Either file a separate ticket and reference it, or drive the checkbox via keyboard/dispatched events and unskip.
severity: significant
resolution: Filed BUG-9RANL to track the test-harness gap. Updated the skip comment to reference the bug and clarify the gap is harness-only, not product-level.
status: addressed
---
