---
id: RR-2KNE
type: review-response
title: Counter-reset test asserts a negative that can pass for wrong reason
finding: '''resets data-cb-idx counter per render'' asserts second).not.toContain(''data-cb-idx="2"''). True if counter resets, OR if counter is incremented differently, OR if renderer stopped emitting checkboxes. Loosely worded.'
severity: nit
resolution: 'Replaced the negative `not.toContain(''data-cb-idx="2"'')` assertion with an exact-count match: `expect(matches).toEqual([''data-cb-idx="0"'', ''data-cb-idx="1"''])`. Now the test fails if the counter increments wrong, or if checkboxes stop emitting entirely.'
status: addressed
---
