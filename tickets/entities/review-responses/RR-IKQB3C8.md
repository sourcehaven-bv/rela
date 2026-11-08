---
id: RR-IKQB3C8
type: review-response
title: Registry tests could not fail
finding: Tests using zero Services asserted Close()==nil, which holds whether or not the registry closed the value.
severity: minor
resolution: Ported the eviction, double-release and sibling tests to closeCounter and assert close counts.
status: addressed
---
