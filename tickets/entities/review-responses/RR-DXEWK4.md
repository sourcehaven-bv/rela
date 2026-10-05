---
id: RR-DXEWK4
type: review-response
title: History fixme cannot detect a cross-face restore
finding: The policy was draft-only, so the published-face null check held whether or not restore leaked across faces.
severity: critical
resolution: The test publishes the policy before restoring, then asserts the draft is back at v1 and the published face keeps the later title. Added test.slow().
status: addressed
---

The policy was draft-only, so the published-face null check held whether or not
restore leaked across faces.
