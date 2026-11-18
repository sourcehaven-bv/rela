---
id: RR-KDOUBIQ
type: review-response
title: Negative e2e test may be vacuous
finding: The e2e test that asserts Add to pile is absent without staff.manage had no positive control under acl.yaml, so it would pass if the button never rendered under any acl.yaml.
severity: significant
resolution: Added a staff fixture whose role holds staff.manage; the test asserts the button is visible there.
status: addressed
---
