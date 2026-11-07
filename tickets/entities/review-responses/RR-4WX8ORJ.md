---
id: RR-4WX8ORJ
type: review-response
title: 'Design: sqlite heal lock order'
finding: Running the heal under versionMu and then taking writeMu adds a lock-order risk.
severity: minor
resolution: 'Plan changed: the sqlite heal runs before versionMu is taken.'
status: addressed
---
