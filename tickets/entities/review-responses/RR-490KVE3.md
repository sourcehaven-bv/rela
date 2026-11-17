---
id: RR-490KVE3
type: review-response
title: Case-name regex was not anchored
finding: The coverage test matched t.case( anywhere, including in comments or strings.
severity: minor
resolution: Regex anchored to the line start with (?m)^t\.case.
status: addressed
---
