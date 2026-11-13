---
id: RR-0GUKAY9
type: review-response
title: git:sync grep guard too loose
finding: grep for "git:sync" anywhere passes even if the manager sed stopped matching. Grep for the full replaced line.
severity: nit
resolution: The guard greps the full replaced manager line, and a second guard checks the new assignment.
status: addressed
---
