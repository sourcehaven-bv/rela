---
id: RR-CSGFVEB
type: review-response
title: Duplicate address-to-title loop in Tasks
finding: Tasks repeats the shape of nameSubjects for a different type.
severity: minor
reason: The rule itself lives once in titles and WholeDisplayTitle; the two loops only copy fields between different view types. A generic helper would add more code than it removes.
status: wont-fix
---
