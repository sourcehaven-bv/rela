---
id: RR-W2TAPQF
type: review-response
title: flowcfg test may pass vacuously
finding: 'strings.Replace on assignments: inserts nothing if the fixture changes, and the loads case would pass without testing anything.'
severity: minor
resolution: The test now fails when the fixture lacks the line; the realtime test does the same.
status: addressed
---
