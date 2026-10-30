---
id: RR-W42GHH
type: review-response
title: AC 2 read-revoked subtest does not test what its name says, and the reload case is missing
finding: 'live_busy_test.go:279 ''read revoked'' ends with a 200 and an empty page, so it exercises the empty-page rule a second time, not the non-200 path the plan specified. The non-200 rule is covered only by the applyTopic unit rows at :307-317. The plan''s ''type removed from realtime.yaml'' case (reconfigured, then a full round, then not served, then the wake clears) has no test. Fix: rename the subtest to ''membership removed empties the page''. Add a subtest that stores a config without the type, signals h.reconfigured, steps, and asserts busyWakeOf is nil and a later run move does not replay the board.'
severity: minor
resolution: 'Fixed: renamed the subtest to membership removed empties the page. Added type removed from realtime.yaml: it stores a config with the run type renamed, calls policyChanged, steps until the wake is nil, then asserts a run Mark no longer replays the board.'
status: addressed
---
