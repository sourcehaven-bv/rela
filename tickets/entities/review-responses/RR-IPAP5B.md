---
id: RR-IPAP5B
type: review-response
title: Unknown face and wildcard-with-face grants still load
finding: policy@typo and *@draft load without error and grant nothing.
severity: minor
reason: Outside the PR 4 scope in the stage 3 design (sections 7 and 20.5), which covers bare and alias grants on faced types. A face-name check belongs with the state-grant validation follow-up.
status: deferred
---
